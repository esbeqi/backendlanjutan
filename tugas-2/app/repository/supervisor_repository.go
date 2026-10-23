package repository

import (
	"context"
	"errors"
	"fmt"

	"api-students/app/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrStudentNotFound    = errors.New("mahasiswa tidak ditemukan")
	ErrSupervisorNotFound = errors.New("supervisor tidak ditemukan")
	ErrAlreadyAssigned    = errors.New("mahasiswa sudah memiliki supervisor")
	ErrSupervisorFull     = errors.New("kuota supervisor penuh")
)

type SupervisorRepository interface {
	WithTx(tx DBTX) SupervisorRepository
	LockStudent(ctx context.Context, studentID int) error
	FindSupervisionByStudent(ctx context.Context, studentID int) (model.Supervision, error)
	IncrementSupervisedCount(ctx context.Context, supervisorID int) (model.Supervisor, error)
	CreateSupervision(ctx context.Context, supervision model.Supervision) error
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
		SET supervised_count = supervised_count + 1
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
