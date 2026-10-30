package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
	"latihan-fiber/app/service"
	"latihan-fiber/database"
	"latihan-fiber/helper"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if err := godotenv.Load(filepath.Join("..", "..", ".env")); err != nil {
		t.Skip(".env lokal tidak tersedia untuk uji integrasi")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	pool, err := database.NewPool(ctx)
	if err != nil {
		t.Skip("PostgreSQL tidak tersedia untuk uji integrasi")
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestDropWritesDroppedEvent(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var userID, courseID int
	if err := pool.QueryRow(ctx, `INSERT INTO users(username,email,password,role,is_active) VALUES($1,$2,'x','user',true) RETURNING id`, "mod11drop"+suffix, "mod11drop"+suffix+"@example.invalid").Scan(&userID); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO courses(code,name,credits,quota,enrolled_count) VALUES($1,'Modul 11 disposable',1,1,1) RETURNING id`, "T"+suffix[len(suffix)-15:]).Scan(&courseID); err != nil {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
		t.Fatalf("create test course: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO enrollments(course_id,user_id) VALUES($1,$2)`, courseID, userID); err != nil {
		t.Fatalf("create test enrollment: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM outbox WHERE payload->>'user_id'=$1`, fmt.Sprint(userID))
		_, _ = pool.Exec(context.Background(), `DELETE FROM courses WHERE id=$1`, courseID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	})

	svc := service.NewCourseService(repository.NewCourseRepository(pool), repository.NewOutboxRepository(pool), repository.NewTxManager(pool))
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(helper.LocalsAuthUser, model.AuthUser{UserID: userID, Username: "probe", Role: "user"})
		return c.Next()
	})
	app.Delete("/courses/:id/enroll", svc.Drop)
	resp, err := app.Test(httptest.NewRequest("DELETE", fmt.Sprintf("/courses/%d/enroll", courseID), nil))
	if err != nil || resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("drop response status=%v err=%v", statusCode(resp), err)
	}
	var eventType string
	if err := pool.QueryRow(ctx, `SELECT event_type FROM outbox WHERE payload->>'user_id'=$1 AND payload->>'course_id'=$2`, fmt.Sprint(userID), fmt.Sprint(courseID)).Scan(&eventType); err != nil {
		t.Fatalf("read drop event: %v", err)
	}
	if eventType != model.EventEnrollmentDropped {
		t.Fatalf("drop event type = %q, want %q", eventType, model.EventEnrollmentDropped)
	}
}

func statusCode(resp *http.Response) any {
	if resp == nil {
		return "no response"
	}
	return resp.StatusCode
}

type temporaryFailMailer struct{}

func (temporaryFailMailer) Send(context.Context, model.Message) error {
	return helper.ErrTemporaryDelivery
}

func TestTemporaryMailerFailureRollsBackNotificationClaim(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var userID int
	if err := pool.QueryRow(ctx, `INSERT INTO users(username,email,password,role,is_active) VALUES($1,$2,'x','user',true) RETURNING id`, "mod11worker"+suffix, "mod11worker"+suffix+"@example.invalid").Scan(&userID); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	payload, _ := json.Marshal(model.EnrollmentEvent{UserID: userID, CourseID: 1, CourseCode: "TST", CourseName: "Disposable"})
	var outboxID int64
	if err := pool.QueryRow(ctx, `INSERT INTO outbox(event_type,payload) VALUES($1,$2) RETURNING id`, model.EventEnrollmentCreated, payload).Scan(&outboxID); err != nil {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
		t.Fatalf("create test outbox event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE outbox_id=$1`, outboxID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM outbox WHERE id=$1`, outboxID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	})
	job, _ := json.Marshal(model.NotificationJob{OutboxID: outboxID, EventType: model.EventEnrollmentCreated, Payload: payload})
	worker := service.NewNotificationWorker(repository.NewUserRepository(pool), repository.NewNotificationRepository(pool), repository.NewTxManager(pool), temporaryFailMailer{}, nil)
	err := worker.HandleSend(ctx, asynq.NewTask(model.TaskSendNotification, job))
	if !errors.Is(err, helper.ErrTemporaryDelivery) {
		t.Fatalf("HandleSend error = %v, want temporary delivery failure", err)
	}
	var claims int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE outbox_id=$1`, outboxID).Scan(&claims); err != nil {
		t.Fatalf("count notification claims: %v", err)
	}
	if claims != 0 {
		t.Fatalf("notification claim remained after failed send: count=%d", claims)
	}
}

type countingMailer struct{ sends atomic.Int32 }

func (m *countingMailer) Send(context.Context, model.Message) error {
	m.sends.Add(1)
	return nil
}

func TestDuplicateTasksCreateOneNotificationAndEmail(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var userID int
	if err := pool.QueryRow(ctx, `INSERT INTO users(username,email,password,role,is_active) VALUES($1,$2,'x','user',true) RETURNING id`, "mod11dupe"+suffix, "mod11dupe"+suffix+"@example.invalid").Scan(&userID); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	payload, _ := json.Marshal(model.EnrollmentEvent{UserID: userID, CourseID: 1, CourseCode: "TST", CourseName: "Disposable"})
	var outboxID int64
	if err := pool.QueryRow(ctx, `INSERT INTO outbox(event_type,payload) VALUES($1,$2) RETURNING id`, model.EventEnrollmentCreated, payload).Scan(&outboxID); err != nil {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
		t.Fatalf("create test outbox event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE outbox_id=$1`, outboxID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM outbox WHERE id=$1`, outboxID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	})
	job, _ := json.Marshal(model.NotificationJob{OutboxID: outboxID, EventType: model.EventEnrollmentCreated, Payload: payload})
	mailer := &countingMailer{}
	worker := service.NewNotificationWorker(repository.NewUserRepository(pool), repository.NewNotificationRepository(pool), repository.NewTxManager(pool), mailer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	task := asynq.NewTask(model.TaskSendNotification, job)
	const copies = 6
	errs := make(chan error, copies)
	var wg sync.WaitGroup
	for i := 0; i < copies; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- worker.HandleSend(ctx, task)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("duplicate task handler: %v", err)
		}
	}
	var claims int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE outbox_id=$1`, outboxID).Scan(&claims); err != nil {
		t.Fatalf("count notification claims: %v", err)
	}
	if claims != 1 || mailer.sends.Load() != 1 {
		t.Fatalf("duplicate handling produced claims=%d sends=%d; want 1 and 1", claims, mailer.sends.Load())
	}
}

func TestPermanentMailerFailureIsSkippedWithoutClaim(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var userID int
	if err := pool.QueryRow(ctx, `INSERT INTO users(username,email,password,role,is_active) VALUES($1,$2,'x','user',true) RETURNING id`, "mod11perm"+suffix, "mod11perm"+suffix+"@example.invalid").Scan(&userID); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	payload, _ := json.Marshal(model.EnrollmentEvent{UserID: userID, CourseID: 1, CourseCode: "TST", CourseName: "Disposable"})
	var outboxID int64
	if err := pool.QueryRow(ctx, `INSERT INTO outbox(event_type,payload) VALUES($1,$2) RETURNING id`, model.EventEnrollmentCreated, payload).Scan(&outboxID); err != nil {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
		t.Fatalf("create test outbox event: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE outbox_id=$1`, outboxID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM outbox WHERE id=$1`, outboxID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	})
	job, _ := json.Marshal(model.NotificationJob{OutboxID: outboxID, EventType: model.EventEnrollmentCreated, Payload: payload})
	mailer, err := helper.NewLogMailer(filepath.Join(t.TempDir(), "mail.log"), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	worker := service.NewNotificationWorker(repository.NewUserRepository(pool), repository.NewNotificationRepository(pool), repository.NewTxManager(pool), mailer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	err = worker.HandleSend(ctx, asynq.NewTask(model.TaskSendNotification, job))
	if !errors.Is(err, asynq.SkipRetry) || !errors.Is(err, helper.ErrPermanentDelivery) {
		t.Fatalf("permanent mail failure error = %v; want SkipRetry and permanent delivery", err)
	}
	var claims int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM notifications WHERE outbox_id=$1`, outboxID).Scan(&claims); err != nil {
		t.Fatalf("count notification claims: %v", err)
	}
	if claims != 0 {
		t.Fatalf("permanent mail failure left claim count=%d; want 0", claims)
	}
}
