package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"latihan-fiber/app/model"
)

var ErrEnrollmentNotFound = errors.New("enrollment tidak ditemukan")
var ErrDuplicateEnrollment = errors.New("user sudah mengambil course ini")

type EnrollmentRepository interface {
	ListByUser(context.Context, DBTX, int) ([]model.Enrollment, error)
	Find(context.Context, DBTX, int64, int) (model.Enrollment, error)
	FindForUpdate(context.Context, DBTX, int64, int) (model.Enrollment, error)
	Create(context.Context, DBTX, int, int) (model.Enrollment, error)
	Delete(context.Context, DBTX, int64, int) error
}

type postgresEnrollmentRepository struct{}

func NewEnrollmentRepository(_ DBTX) EnrollmentRepository {
	return &postgresEnrollmentRepository{}
}

func (r *postgresEnrollmentRepository) ListByUser(ctx context.Context, db DBTX, userID int) ([]model.Enrollment, error) {
	rows, err := db.Query(ctx, `SELECT id,course_id,user_id,created_at
		FROM enrollments WHERE user_id=$1 ORDER BY created_at DESC,id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user enrollments: %w", err)
	}
	defer rows.Close()
	items := make([]model.Enrollment, 0)
	for rows.Next() {
		var item model.Enrollment
		if err := rows.Scan(&item.ID, &item.CourseID, &item.UserID, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan enrollment: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enrollments: %w", err)
	}
	return items, nil
}

func (r *postgresEnrollmentRepository) FindForUpdate(ctx context.Context, db DBTX, id int64, userID int) (model.Enrollment, error) {
	return findEnrollment(ctx, db, id, userID, true)
}

func (r *postgresEnrollmentRepository) Find(ctx context.Context, db DBTX, id int64, userID int) (model.Enrollment, error) {
	return findEnrollment(ctx, db, id, userID, false)
}

func findEnrollment(ctx context.Context, db DBTX, id int64, userID int, lock bool) (model.Enrollment, error) {
	var item model.Enrollment
	query := `SELECT id,course_id,user_id,created_at FROM enrollments WHERE id=$1 AND user_id=$2`
	if lock {
		query += " FOR UPDATE"
	}
	err := db.QueryRow(ctx, query, id, userID).
		Scan(&item.ID, &item.CourseID, &item.UserID, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Enrollment{}, ErrEnrollmentNotFound
	}
	if err != nil {
		return model.Enrollment{}, fmt.Errorf("find enrollment: %w", err)
	}
	return item, nil
}

func (r *postgresEnrollmentRepository) Create(ctx context.Context, db DBTX, courseID, userID int) (model.Enrollment, error) {
	var item model.Enrollment
	err := db.QueryRow(ctx, `INSERT INTO enrollments(course_id,user_id)
		VALUES($1,$2) RETURNING id,course_id,user_id,created_at`, courseID, userID).
		Scan(&item.ID, &item.CourseID, &item.UserID, &item.CreatedAt)
	if isUniqueViolation(err) {
		return model.Enrollment{}, ErrDuplicateEnrollment
	}
	if err != nil {
		return model.Enrollment{}, fmt.Errorf("create enrollment: %w", err)
	}
	return item, nil
}

func (r *postgresEnrollmentRepository) Delete(ctx context.Context, db DBTX, id int64, userID int) error {
	tag, err := db.Exec(ctx, `DELETE FROM enrollments WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete enrollment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrEnrollmentNotFound
	}
	return nil
}
