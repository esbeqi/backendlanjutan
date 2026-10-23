package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"api-students/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrStudentNotFound           = errors.New("mahasiswa tidak ditemukan")
	ErrSupervisorNotFound        = errors.New("supervisor tidak ditemukan")
	ErrAlreadyAssigned           = errors.New("mahasiswa sudah memiliki supervisor")
	ErrSupervisorFull            = errors.New("kuota supervisor penuh")
	ErrNotAssigned               = errors.New("mahasiswa belum memiliki supervisor")
	ErrSameSupervisor            = errors.New("supervisor tujuan sama dengan supervisor saat ini")
	ErrSupervisorVersion         = errors.New("versi supervisor sudah berubah")
	ErrQuotaBelowSupervisedCount = errors.New("kuota tidak boleh lebih kecil dari jumlah mahasiswa terbimbing")
)

type SupervisorRepository interface {
	WithTx(tx DBTX) SupervisorRepository
	LockStudent(ctx context.Context, studentID int) error
	FindSupervisionByStudent(ctx context.Context, studentID int) (model.Supervision, error)
	IncrementSupervisedCount(ctx context.Context, supervisorID int) (model.Supervisor, error)
	CreateSupervision(ctx context.Context, supervision model.Supervision) error
	LockSupervisors(ctx context.Context, supervisorIDs ...int) error
	DecrementSupervisedCount(ctx context.Context, supervisorID int) error
	SwitchSupervision(ctx context.Context, studentID, oldSupervisorID, newSupervisorID int) error
	UpdateSupervisor(ctx context.Context, id, expectedVersion int, name *string, quota *int) (model.Supervisor, error)
}

type supervisorPostgresRepository struct {
	db DBTX
}

func NewSupervisorRepository(db DBTX) SupervisorRepository {
	return &supervisorPostgresRepository{db: db}
}

func (r *supervisorPostgresRepository) WithTx(tx DBTX) SupervisorRepository {
	return &supervisorPostgresRepository{db: tx}
}

func (r *supervisorPostgresRepository) LockStudent(ctx context.Context, studentID int) error {
	var id int
	err := r.db.QueryRow(ctx, `SELECT id FROM students WHERE id = $1 FOR UPDATE`, studentID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrStudentNotFound
	}
	if err != nil {
		return fmt.Errorf("lock student row: %w", err)
	}
	return nil
}

func (r *supervisorPostgresRepository) FindSupervisionByStudent(ctx context.Context, studentID int) (model.Supervision, error) {
	var supervision model.Supervision
	err := r.db.QueryRow(ctx, `
		SELECT student_id, supervisor_id
		FROM supervisions
		WHERE student_id = $1
	`, studentID).Scan(&supervision.StudentID, &supervision.SupervisorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Supervision{}, nil
	}
	if err != nil {
		return model.Supervision{}, fmt.Errorf("find student supervision: %w", err)
	}
	return supervision, nil
}

func (r *supervisorPostgresRepository) IncrementSupervisedCount(ctx context.Context, supervisorID int) (model.Supervisor, error) {
	var supervisor model.Supervisor
	err := r.db.QueryRow(ctx, `
		UPDATE supervisors
		SET supervised_count = supervised_count + 1,
		    version = version + 1
		WHERE id = $1 AND supervised_count < quota
		RETURNING id, name, quota, supervised_count, version
	`, supervisorID).Scan(
		&supervisor.ID,
		&supervisor.Name,
		&supervisor.Quota,
		&supervisor.SupervisedCount,
		&supervisor.Version,
	)
	if err == nil {
		return supervisor, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Supervisor{}, fmt.Errorf("increment supervised count: %w", err)
	}

	err = r.db.QueryRow(ctx, `SELECT id FROM supervisors WHERE id = $1`, supervisorID).Scan(&supervisor.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Supervisor{}, ErrSupervisorNotFound
	}
	if err != nil {
		return model.Supervisor{}, fmt.Errorf("check supervisor after conditional update: %w", err)
	}
	return model.Supervisor{}, ErrSupervisorFull
}

func (r *supervisorPostgresRepository) CreateSupervision(ctx context.Context, supervision model.Supervision) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO supervisions (student_id, supervisor_id)
		VALUES ($1, $2)
	`, supervision.StudentID, supervision.SupervisorID)
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505" && pgErr.ConstraintName == "supervisions_student_id_unique":
			return ErrAlreadyAssigned
		case pgErr.Code == "23503" && pgErr.ConstraintName == "supervisions_student_id_fkey":
			return ErrStudentNotFound
		case pgErr.Code == "23503" && pgErr.ConstraintName == "supervisions_supervisor_id_fkey":
			return ErrSupervisorNotFound
		}
	}
	return fmt.Errorf("create supervision: %w", err)
}

// LockSupervisors mengunci setiap supervisor dengan urutan ID yang konsisten.
func (r *supervisorPostgresRepository) LockSupervisors(ctx context.Context, supervisorIDs ...int) error {
	ids := append([]int(nil), supervisorIDs...)
	sort.Ints(ids)
	previousID := 0
	for _, id := range ids {
		if id == previousID {
			continue
		}
		var lockedID int
		err := r.db.QueryRow(ctx, `SELECT id FROM supervisors WHERE id = $1 FOR UPDATE`, id).Scan(&lockedID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSupervisorNotFound
		}
		if err != nil {
			return fmt.Errorf("lock supervisor row: %w", err)
		}
		previousID = id
	}
	return nil
}

func (r *supervisorPostgresRepository) DecrementSupervisedCount(ctx context.Context, supervisorID int) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE supervisors
		SET supervised_count = supervised_count - 1,
		    version = version + 1
		WHERE id = $1 AND supervised_count > 0
	`, supervisorID)
	if err != nil {
		return fmt.Errorf("decrement supervised count: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("decrement supervised count: inconsistent counter for supervisor %d", supervisorID)
	}
	return nil
}

func (r *supervisorPostgresRepository) SwitchSupervision(ctx context.Context, studentID, oldSupervisorID, newSupervisorID int) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE supervisions
		SET supervisor_id = $1
		WHERE student_id = $2 AND supervisor_id = $3
	`, newSupervisorID, studentID, oldSupervisorID)
	if err != nil {
		return fmt.Errorf("switch supervision: %w", err)
	}
	if cmd.RowsAffected() != 1 {
		return ErrNotAssigned
	}
	return nil
}

func (r *supervisorPostgresRepository) UpdateSupervisor(ctx context.Context, id, expectedVersion int, name *string, quota *int) (model.Supervisor, error) {
	var supervisor model.Supervisor
	err := r.db.QueryRow(ctx, `
		UPDATE supervisors
		SET name = COALESCE($1, name),
		    quota = COALESCE($2, quota),
		    version = version + 1
		WHERE id = $3
		  AND version = $4
		  AND COALESCE($2, quota) >= supervised_count
		RETURNING id, name, quota, supervised_count, version
	`, name, quota, id, expectedVersion).Scan(
		&supervisor.ID,
		&supervisor.Name,
		&supervisor.Quota,
		&supervisor.SupervisedCount,
		&supervisor.Version,
	)
	if err == nil {
		return supervisor, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Supervisor{}, fmt.Errorf("update supervisor: %w", err)
	}

	var currentVersion int
	var currentQuota, supervisedCount int
	err = r.db.QueryRow(ctx, `
		SELECT version, quota, supervised_count
		FROM supervisors
		WHERE id = $1
	`, id).Scan(&currentVersion, &currentQuota, &supervisedCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Supervisor{}, ErrSupervisorNotFound
	}
	if err != nil {
		return model.Supervisor{}, fmt.Errorf("check supervisor after conditional update: %w", err)
	}
	if currentVersion != expectedVersion {
		return model.Supervisor{}, ErrSupervisorVersion
	}
	if quota != nil && *quota < supervisedCount {
		return model.Supervisor{}, ErrQuotaBelowSupervisedCount
	}
	return model.Supervisor{}, fmt.Errorf("conditional supervisor update affected no rows")
}
