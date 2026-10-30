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
	ErrCourseNotFound           = errors.New("kelas tidak ditemukan")
	ErrCourseVersion            = errors.New("versi kelas sudah berubah")
	ErrCourseQuotaBelowEnrolled = errors.New("kuota kelas lebih kecil dari peserta terdaftar")
	ErrCourseFull               = errors.New("kuota kelas penuh")
	ErrAlreadyEnrolled          = errors.New("mahasiswa sudah terdaftar di kelas")
	ErrNotEnrolled              = errors.New("mahasiswa tidak terdaftar di kelas")
	ErrStudentNotOwned          = errors.New("mahasiswa tidak dimiliki user ini")
	ErrRecipientUnavailable     = errors.New("peserta kelas tidak memiliki alamat email aktif")
)

type CourseRepository interface {
	WithTx(tx DBTX) CourseRepository
	FindByID(ctx context.Context, id int) (model.Course, error)
	LockByID(ctx context.Context, id int) (model.Course, error)
	UpdateQuota(ctx context.Context, id, version, quota int) (model.Course, error)
	Recipients(ctx context.Context, courseID int) ([]model.NotificationRecipient, error)
	Enroll(ctx context.Context, courseID, userID int) (model.Course, model.NotificationRecipient, error)
	Drop(ctx context.Context, courseID, userID int) (model.Course, model.NotificationRecipient, error)
}

type coursePostgresRepository struct{ db DBTX }

func NewCourseRepository(db DBTX) CourseRepository { return &coursePostgresRepository{db: db} }

func (r *coursePostgresRepository) WithTx(tx DBTX) CourseRepository {
	return &coursePostgresRepository{db: tx}
}

func (r *coursePostgresRepository) FindByID(ctx context.Context, id int) (model.Course, error) {
	return scanCourse(r.db.QueryRow(ctx, `
		SELECT id, code, name, quota, enrolled_count, version, created_at
		FROM courses WHERE id = $1`, id))
}

func (r *coursePostgresRepository) LockByID(ctx context.Context, id int) (model.Course, error) {
	return scanCourse(r.db.QueryRow(ctx, `
		SELECT id, code, name, quota, enrolled_count, version, created_at
		FROM courses WHERE id = $1 FOR UPDATE`, id))
}

func scanCourse(row pgx.Row) (model.Course, error) {
	var course model.Course
	if err := row.Scan(&course.ID, &course.Code, &course.Name, &course.Quota, &course.EnrolledCount, &course.Version, &course.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Course{}, ErrCourseNotFound
		}
		return model.Course{}, fmt.Errorf("membaca kelas: %w", err)
	}
	return course, nil
}

func (r *coursePostgresRepository) UpdateQuota(ctx context.Context, id, version, quota int) (model.Course, error) {
	course, err := scanCourse(r.db.QueryRow(ctx, `
		UPDATE courses
		SET quota = $1, version = version + 1
		WHERE id = $2 AND version = $3 AND $1 >= enrolled_count
		RETURNING id, code, name, quota, enrolled_count, version, created_at`, quota, id, version))
	if err == nil {
		return course, nil
	}
	if !errors.Is(err, ErrCourseNotFound) {
		return model.Course{}, err
	}
	current, lookupErr := r.FindByID(ctx, id)
	if errors.Is(lookupErr, ErrCourseNotFound) {
		return model.Course{}, ErrCourseNotFound
	}
	if lookupErr != nil {
		return model.Course{}, lookupErr
	}
	if current.Version != version {
		return model.Course{}, ErrCourseVersion
	}
	if quota < current.EnrolledCount {
		return model.Course{}, ErrCourseQuotaBelowEnrolled
	}
	return model.Course{}, fmt.Errorf("update kelas tidak mengubah baris")
}

func (r *coursePostgresRepository) Recipients(ctx context.Context, courseID int) ([]model.NotificationRecipient, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id, COALESCE(NULLIF(BTRIM(u.email), ''), '')
		FROM enrollments e
		JOIN users u ON u.id = e.user_id AND u.is_active = TRUE
		WHERE e.course_id = $1
		ORDER BY u.id`, courseID)
	if err != nil {
		return nil, fmt.Errorf("mengambil peserta kelas: %w", err)
	}
	defer rows.Close()

	recipients := make([]model.NotificationRecipient, 0)
	for rows.Next() {
		var recipient model.NotificationRecipient
		if err := rows.Scan(&recipient.UserID, &recipient.Email); err != nil {
			return nil, fmt.Errorf("membaca peserta kelas: %w", err)
		}
		if recipient.Email == "" {
			return nil, fmt.Errorf("peserta %d: %w", recipient.UserID, ErrRecipientUnavailable)
		}
		recipients = append(recipients, recipient)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mengiterasi peserta kelas: %w", err)
	}
	return recipients, nil
}

func (r *coursePostgresRepository) Enroll(ctx context.Context, courseID, userID int) (model.Course, model.NotificationRecipient, error) {
	course, err := r.LockByID(ctx, courseID)
	if err != nil {
		return model.Course{}, model.NotificationRecipient{}, err
	}
	var email string
	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(BTRIM(email), ''), '') FROM users
		WHERE id = $1 AND is_active = TRUE`, userID).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Course{}, model.NotificationRecipient{}, ErrStudentNotOwned
	}
	if err != nil {
		return model.Course{}, model.NotificationRecipient{}, fmt.Errorf("memeriksa pemilik mahasiswa: %w", err)
	}
	if email == "" {
		return model.Course{}, model.NotificationRecipient{}, ErrStudentNotOwned
	}
	if course.EnrolledCount >= course.Quota {
		return model.Course{}, model.NotificationRecipient{}, ErrCourseFull
	}
	if _, err := r.db.Exec(ctx, `INSERT INTO enrollments(course_id, user_id) VALUES($1,$2)`, courseID, userID); err != nil {
		if isUniqueViolation(err) {
			return model.Course{}, model.NotificationRecipient{}, ErrAlreadyEnrolled
		}
		return model.Course{}, model.NotificationRecipient{}, fmt.Errorf("mendaftarkan mahasiswa ke kelas: %w", err)
	}
	course, err = scanCourse(r.db.QueryRow(ctx, `
		UPDATE courses SET enrolled_count=enrolled_count+1, version=version+1
		WHERE id=$1 AND enrolled_count < quota
		RETURNING id, code, name, quota, enrolled_count, version, created_at`, courseID))
	if err != nil {
		return model.Course{}, model.NotificationRecipient{}, err
	}
	return course, model.NotificationRecipient{UserID: userID, Email: email}, nil
}

func (r *coursePostgresRepository) Drop(ctx context.Context, courseID, userID int) (model.Course, model.NotificationRecipient, error) {
	course, err := r.LockByID(ctx, courseID)
	if err != nil {
		return model.Course{}, model.NotificationRecipient{}, err
	}
	var email string
	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(BTRIM(u.email), ''), '')
		FROM users u
		WHERE u.id=$1 AND u.is_active=TRUE`, userID).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Course{}, model.NotificationRecipient{}, ErrStudentNotOwned
	}
	if err != nil {
		return model.Course{}, model.NotificationRecipient{}, fmt.Errorf("memeriksa pemilik mahasiswa: %w", err)
	}
	if email == "" {
		return model.Course{}, model.NotificationRecipient{}, ErrRecipientUnavailable
	}
	cmd, err := r.db.Exec(ctx, `DELETE FROM enrollments WHERE course_id=$1 AND user_id=$2`, courseID, userID)
	if err != nil {
		return model.Course{}, model.NotificationRecipient{}, fmt.Errorf("membatalkan pendaftaran: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return model.Course{}, model.NotificationRecipient{}, ErrNotEnrolled
	}
	course, err = scanCourse(r.db.QueryRow(ctx, `
		UPDATE courses SET enrolled_count=enrolled_count-1, version=version+1
		WHERE id=$1 AND enrolled_count > 0
		RETURNING id, code, name, quota, enrolled_count, version, created_at`, courseID))
	if err != nil {
		return model.Course{}, model.NotificationRecipient{}, err
	}
	return course, model.NotificationRecipient{UserID: userID, Email: email}, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
