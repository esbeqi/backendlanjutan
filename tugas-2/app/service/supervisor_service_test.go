package service

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"api-students/app/model"
	"api-students/app/repository"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type testDBTX struct{}

func (testDBTX) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (testDBTX) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (testDBTX) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }

type testTxManager struct {
	callbackFailed bool
}

func (m *testTxManager) WithTx(_ context.Context, fn func(repository.DBTX) error) error {
	err := fn(testDBTX{})
	m.callbackFailed = err != nil
	return err
}

type testSupervisorRepository struct {
	studentErr   error
	existing     bool
	current      model.Supervision
	lookupErr    error
	incrementErr error
	createErr    error
	lockError    error
	decrementErr error
	switchErr    error
	calls        []string
	lockedIDs    []int
	txReceived   repository.DBTX
}

func (r *testSupervisorRepository) WithTx(tx repository.DBTX) repository.SupervisorRepository {
	r.txReceived = tx
	return r
}
func (r *testSupervisorRepository) LockStudent(context.Context, int) error {
	r.calls = append(r.calls, "lock_student")
	return r.studentErr
}
func (r *testSupervisorRepository) FindSupervisionByStudent(context.Context, int) (model.Supervision, error) {
	r.calls = append(r.calls, "find_supervision")
	if r.lookupErr != nil {
		return model.Supervision{}, r.lookupErr
	}
	if r.existing {
		if r.current.StudentID == 0 {
			r.current = model.Supervision{StudentID: 9, SupervisorID: 8}
		}
		return r.current, nil
	}
	return model.Supervision{}, nil
}
func (r *testSupervisorRepository) IncrementSupervisedCount(context.Context, int) (model.Supervisor, error) {
	r.calls = append(r.calls, "increment")
	return model.Supervisor{}, r.incrementErr
}
func (r *testSupervisorRepository) CreateSupervision(context.Context, model.Supervision) error {
	r.calls = append(r.calls, "create")
	return r.createErr
}
func (r *testSupervisorRepository) LockSupervisors(_ context.Context, supervisorIDs ...int) error {
	r.calls = append(r.calls, "lock_supervisors")
	r.lockedIDs = append([]int(nil), supervisorIDs...)
	return r.lockError
}
func (r *testSupervisorRepository) DecrementSupervisedCount(context.Context, int) error {
	r.calls = append(r.calls, "decrement")
	return r.decrementErr
}
func (r *testSupervisorRepository) SwitchSupervision(context.Context, int, int, int) error {
	r.calls = append(r.calls, "switch")
	return r.switchErr
}

func TestSupervisorAssign(t *testing.T) {
	tests := []struct {
		name                string
		repo                *testSupervisorRepository
		wantStatus          int
		wantCallbackFailure bool
		wantCalls           string
		internalError       string
	}{
		{name: "student missing", repo: &testSupervisorRepository{studentErr: repository.ErrStudentNotFound}, wantStatus: fiber.StatusNotFound, wantCallbackFailure: true, wantCalls: "lock_student"},
		{name: "already assigned", repo: &testSupervisorRepository{existing: true}, wantStatus: fiber.StatusConflict, wantCallbackFailure: true, wantCalls: "lock_student,find_supervision"},
		{name: "supervisor missing", repo: &testSupervisorRepository{incrementErr: repository.ErrSupervisorNotFound}, wantStatus: fiber.StatusNotFound, wantCallbackFailure: true, wantCalls: "lock_student,find_supervision,increment"},
		{name: "quota full", repo: &testSupervisorRepository{incrementErr: repository.ErrSupervisorFull}, wantStatus: fiber.StatusConflict, wantCallbackFailure: true, wantCalls: "lock_student,find_supervision,increment"},
		{name: "insert fails without leaking internal error", repo: &testSupervisorRepository{createErr: errors.New("private database detail")}, wantStatus: fiber.StatusInternalServerError, wantCallbackFailure: true, wantCalls: "lock_student,find_supervision,increment,create", internalError: "private database detail"},
		{name: "lookup fails without leaking internal error", repo: &testSupervisorRepository{lookupErr: errors.New("private query detail")}, wantStatus: fiber.StatusInternalServerError, wantCallbackFailure: true, wantCalls: "lock_student,find_supervision", internalError: "private query detail"},
		{name: "success", repo: &testSupervisorRepository{}, wantStatus: fiber.StatusCreated, wantCallbackFailure: false, wantCalls: "lock_student,find_supervision,increment,create"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &testTxManager{}
			app := fiber.New()
			app.Post("/supervisors/:id/assign", NewSupervisorService(tt.repo, manager).Assign)
			request := httptest.NewRequest("POST", "/supervisors/7/assign", strings.NewReader(`{"student_id":9}`))
			request.Header.Set("Content-Type", "application/json")
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.wantStatus)
			}
			if manager.callbackFailed != tt.wantCallbackFailure {
				t.Fatalf("callbackFailed = %t, want %t", manager.callbackFailed, tt.wantCallbackFailure)
			}
			if got := strings.Join(tt.repo.calls, ","); got != tt.wantCalls {
				t.Fatalf("calls = %q, want %q", got, tt.wantCalls)
			}
			if tt.repo.txReceived != (testDBTX{}) {
				t.Fatalf("WithTx received %T, want callback transaction", tt.repo.txReceived)
			}
			if tt.internalError != "" {
				body, err := io.ReadAll(response.Body)
				if err != nil {
					t.Fatalf("read response: %v", err)
				}
				if strings.Contains(string(body), tt.internalError) {
					t.Fatalf("response leaked internal error: %s", body)
				}
			}
		})
	}
}

func TestSupervisorSwitch(t *testing.T) {
	const requestBody = `{"student_id":9,"new_supervisor_id":2}`
	tests := []struct {
		name                string
		repo                *testSupervisorRepository
		wantStatus          int
		wantCallbackFailure bool
		wantCalls           string
		wantLockIDs         []int
	}{
		{name: "success", repo: &testSupervisorRepository{existing: true, current: model.Supervision{StudentID: 9, SupervisorID: 8}}, wantStatus: fiber.StatusOK, wantCalls: "lock_student,find_supervision,lock_supervisors,decrement,increment,switch", wantLockIDs: []int{8, 2}},
		{name: "student missing", repo: &testSupervisorRepository{studentErr: repository.ErrStudentNotFound}, wantStatus: fiber.StatusNotFound, wantCallbackFailure: true, wantCalls: "lock_student"},
		{name: "not assigned", repo: &testSupervisorRepository{}, wantStatus: fiber.StatusConflict, wantCallbackFailure: true, wantCalls: "lock_student,find_supervision"},
		{name: "same supervisor", repo: &testSupervisorRepository{existing: true, current: model.Supervision{StudentID: 9, SupervisorID: 2}}, wantStatus: fiber.StatusConflict, wantCallbackFailure: true, wantCalls: "lock_student,find_supervision"},
		{name: "new supervisor full", repo: &testSupervisorRepository{existing: true, current: model.Supervision{StudentID: 9, SupervisorID: 8}, incrementErr: repository.ErrSupervisorFull}, wantStatus: fiber.StatusConflict, wantCallbackFailure: true, wantCalls: "lock_student,find_supervision,lock_supervisors,decrement,increment", wantLockIDs: []int{8, 2}},
		{name: "relation update fails", repo: &testSupervisorRepository{existing: true, current: model.Supervision{StudentID: 9, SupervisorID: 8}, switchErr: errors.New("private update detail")}, wantStatus: fiber.StatusInternalServerError, wantCallbackFailure: true, wantCalls: "lock_student,find_supervision,lock_supervisors,decrement,increment,switch", wantLockIDs: []int{8, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &testTxManager{}
			app := fiber.New()
			app.Post("/supervisions/switch", NewSupervisorService(tt.repo, manager).Switch)
			request := httptest.NewRequest("POST", "/supervisions/switch", strings.NewReader(requestBody))
			request.Header.Set("Content-Type", "application/json")
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.wantStatus)
			}
			if manager.callbackFailed != tt.wantCallbackFailure {
				t.Fatalf("callbackFailed = %t, want %t", manager.callbackFailed, tt.wantCallbackFailure)
			}
			if got := strings.Join(tt.repo.calls, ","); got != tt.wantCalls {
				t.Fatalf("calls = %q, want %q", got, tt.wantCalls)
			}
			if len(tt.repo.lockedIDs) != len(tt.wantLockIDs) {
				t.Fatalf("lockedIDs = %v, want %v", tt.repo.lockedIDs, tt.wantLockIDs)
			}
			for i := range tt.wantLockIDs {
				if tt.repo.lockedIDs[i] != tt.wantLockIDs[i] {
					t.Fatalf("lockedIDs = %v, want %v", tt.repo.lockedIDs, tt.wantLockIDs)
				}
			}
		})
	}
}

func TestSupervisorAssignRejectsInvalidIDs(t *testing.T) {
	for _, path := range []string{"/supervisors/0/assign", "/supervisors/nope/assign"} {
		t.Run(path, func(t *testing.T) {
			manager := &testTxManager{}
			repo := &testSupervisorRepository{}
			app := fiber.New()
			app.Post("/supervisors/:id/assign", NewSupervisorService(repo, manager).Assign)
			response, err := app.Test(httptest.NewRequest("POST", path, strings.NewReader(`{"student_id":9}`)))
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != fiber.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusBadRequest)
			}
			if len(repo.calls) != 0 {
				t.Fatalf("repository called for invalid path: %v", repo.calls)
			}
		})
	}
}

func TestSupervisorAssignRejectsInvalidAndMalformedBodies(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "nonpositive student id", body: `{"student_id":0}`, wantStatus: fiber.StatusUnprocessableEntity},
		{name: "malformed JSON", body: `{"student_id":`, wantStatus: fiber.StatusBadRequest},
		{name: "wrong student id type", body: `{"student_id":"nine"}`, wantStatus: fiber.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &testSupervisorRepository{}
			manager := &testTxManager{}
			app := fiber.New()
			app.Post("/supervisors/:id/assign", NewSupervisorService(repo, manager).Assign)
			request := httptest.NewRequest("POST", "/supervisors/7/assign", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.wantStatus)
			}
			if len(repo.calls) != 0 {
				t.Fatalf("repository called for invalid body: %v", repo.calls)
			}
		})
	}
}
