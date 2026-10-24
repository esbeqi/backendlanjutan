package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"latihan-fiber/app/model"
)

var (
	ErrCourseFull       = errors.New("kuota kelas sudah penuh")
	ErrNotEnrolled      = errors.New("tidak terdaftar di kelas ini")
	ErrVersionConflict  = errors.New("data sudah diubah pihak lain")
	ErrQuotaBelowFilled = errors.New("kuota lebih kecil dari jumlah peserta")
)

type CourseListQuery struct {
	Page   int
	Limit  int
	Offset int
	Search string
}

type CoursePage struct {
	Courses    []model.Course
	Page       int
	Limit      int
	Total      int
	TotalPages int
}

type CourseRepository interface {
	WithTx(tx DBTX) CourseRepository
	List(ctx context.Context, q CourseListQuery) (CoursePage, error)
	FindAll(ctx context.Context) ([]model.Course, error)
	FindByID(ctx context.Context, id int) (model.Course, error)
	IsEnrolled(ctx context.Context, courseID, userID int) (bool, error)
	FindForUpdate(ctx context.Context, id int) (model.Course, error)
	FindByStudent(ctx context.Context, userID int) ([]model.Course, error)
	LockStudent(ctx context.Context, userID int) error
	SumCredits(ctx context.Context, userID int) (int, error)
	InsertEnrollment(ctx context.Context, courseID, userID int) error
	IncrementEnrolled(ctx context.Context, courseID int) (model.Course, error)
	DecrementEnrolled(ctx context.Context, courseID int) (model.Course, error)
	DeleteEnrollment(ctx context.Context, courseID, userID int) error
	LockCourses(ctx context.Context, ids ...int) error
	UpdateQuota(ctx context.Context, id, quota, version int) (model.Course, error)
}

const courseColumns = `id, code, name, credits, quota, enrolled_count, GREATEST(quota-enrolled_count,0), version, created_at`

type coursePostgresRepository struct{ db DBTX }

func NewCourseRepository(db DBTX) CourseRepository {
	return &coursePostgresRepository{db: db}
}

func (r *coursePostgresRepository) WithTx(tx DBTX) CourseRepository {
	return &coursePostgresRepository{db: tx}
}

func (r *coursePostgresRepository) List(ctx context.Context, q CourseListQuery) (CoursePage, error) {
	where := ""
	args := []any{}
	if q.Search != "" {
		args = append(args, "%"+q.Search+"%")
		where = " WHERE code ILIKE $1 OR name ILIKE $1"
	}
	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM courses"+where, args...).Scan(&total); err != nil {
		return CoursePage{}, fmt.Errorf("count courses: %w", err)
	}
	args = append(args, q.Limit, q.Offset)
	query := "SELECT " + courseColumns + " FROM courses" + where + fmt.Sprintf(" ORDER BY code ASC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return CoursePage{}, fmt.Errorf("list courses: %w", err)
	}
	defer rows.Close()
	courses := make([]model.Course, 0)
	for rows.Next() {
		course, err := scanCourse(rows)
		if err != nil {
			return CoursePage{}, err
		}
		courses = append(courses, course)
	}
	if err := rows.Err(); err != nil {
		return CoursePage{}, fmt.Errorf("iterate courses: %w", err)
	}
	pages := 0
	if total > 0 {
		pages = (total + q.Limit - 1) / q.Limit
	}
	return CoursePage{Courses: courses, Page: q.Page, Limit: q.Limit, Total: total, TotalPages: pages}, nil
}

func (r *coursePostgresRepository) FindAll(ctx context.Context) ([]model.Course, error) {
	rows, err := r.db.Query(ctx, "SELECT "+courseColumns+" FROM courses ORDER BY code")
	if err != nil {
		return nil, fmt.Errorf("list courses: %w", err)
	}
	defer rows.Close()
	courses := make([]model.Course, 0)
	for rows.Next() {
		course, err := scanCourse(rows)
		if err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	return courses, rows.Err()
}

func (r *coursePostgresRepository) FindByID(ctx context.Context, id int) (model.Course, error) {
	return r.findByID(ctx, id, false)
}

// IsEnrolled memeriksa status enrollment tanpa mengambil row enrollment.
func (r *coursePostgresRepository) IsEnrolled(ctx context.Context, courseID, userID int) (bool, error) {
	var enrolled bool
	err := r.db.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM enrollments WHERE course_id = $1 AND user_id = $2)",
		courseID,
		userID,
	).Scan(&enrolled)
	if err != nil {
		return false, fmt.Errorf("memeriksa pendaftaran: %w", err)
	}
	return enrolled, nil
}

func (r *coursePostgresRepository) FindForUpdate(ctx context.Context, id int) (model.Course, error) {
	return r.findByID(ctx, id, true)
}

func (r *coursePostgresRepository) findByID(ctx context.Context, id int, lock bool) (model.Course, error) {
	query := "SELECT " + courseColumns + " FROM courses WHERE id=$1"
	if lock {
		query += " FOR UPDATE"
	}
	course, err := scanCourse(r.db.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Course{}, ErrNotFound
	}
	if err != nil {
		return model.Course{}, fmt.Errorf("find course: %w", err)
	}
	return course, nil
}

func (r *coursePostgresRepository) FindByStudent(ctx context.Context, userID int) ([]model.Course, error) {
	rows, err := r.db.Query(ctx, `SELECT c.id, c.code, c.name, c.credits, c.quota, c.enrolled_count,
		GREATEST(c.quota-c.enrolled_count,0), c.version, c.created_at
		FROM courses c JOIN enrollments e ON e.course_id=c.id
		WHERE e.user_id=$1 ORDER BY c.code`, userID)
	if err != nil {
		return nil, fmt.Errorf("list student courses: %w", err)
	}
	defer rows.Close()
	courses := make([]model.Course, 0)
	for rows.Next() {
		course, err := scanCourse(rows)
		if err != nil {
			return nil, err
		}
		courses = append(courses, course)
	}
	return courses, rows.Err()
}

func (r *coursePostgresRepository) LockStudent(ctx context.Context, userID int) error {
	var id int
	err := r.db.QueryRow(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock student: %w", err)
	}
	return nil
}

func (r *coursePostgresRepository) SumCredits(ctx context.Context, userID int) (int, error) {
	var total int
	err := r.db.QueryRow(ctx, `SELECT COALESCE(SUM(c.credits),0)
		FROM enrollments e JOIN courses c ON c.id=e.course_id WHERE e.user_id=$1`, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum student credits: %w", err)
	}
	return total, nil
}

func (r *coursePostgresRepository) InsertEnrollment(ctx context.Context, courseID, userID int) error {
	_, err := r.db.Exec(ctx, "INSERT INTO enrollments(course_id,user_id) VALUES($1,$2)", courseID, userID)
	if pgErrorCode(err) == codeUniqueViolation {
		return ErrDuplicate
	}
	if err != nil {
		return fmt.Errorf("insert enrollment: %w", err)
	}
	return nil
}

func (r *coursePostgresRepository) IncrementEnrolled(ctx context.Context, courseID int) (model.Course, error) {
	course, err := scanCourse(r.db.QueryRow(ctx,
		"UPDATE courses SET enrolled_count=enrolled_count+1 WHERE id=$1 AND enrolled_count<quota RETURNING "+courseColumns, courseID))
	if err == nil {
		return course, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Course{}, fmt.Errorf("increment enrolled count: %w", err)
	}
	if _, findErr := r.FindByID(ctx, courseID); findErr != nil {
		return model.Course{}, findErr
	}
	return model.Course{}, ErrCourseFull
}

func (r *coursePostgresRepository) DecrementEnrolled(ctx context.Context, courseID int) (model.Course, error) {
	course, err := scanCourse(r.db.QueryRow(ctx,
		"UPDATE courses SET enrolled_count=enrolled_count-1 WHERE id=$1 AND enrolled_count>0 RETURNING "+courseColumns, courseID))
	if errors.Is(err, pgx.ErrNoRows) {
		if _, findErr := r.FindByID(ctx, courseID); findErr != nil {
			return model.Course{}, findErr
		}
		return model.Course{}, fmt.Errorf("enrolled count cannot be decremented")
	}
	if err != nil {
		return model.Course{}, fmt.Errorf("decrement enrolled count: %w", err)
	}
	return course, nil
}

func (r *coursePostgresRepository) DeleteEnrollment(ctx context.Context, courseID, userID int) error {
	tag, err := r.db.Exec(ctx, "DELETE FROM enrollments WHERE course_id=$1 AND user_id=$2", courseID, userID)
	if err != nil {
		return fmt.Errorf("delete enrollment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotEnrolled
	}
	return nil
}

func (r *coursePostgresRepository) LockCourses(ctx context.Context, ids ...int) error {
	rows, err := r.db.Query(ctx, "SELECT id FROM courses WHERE id=ANY($1) ORDER BY id FOR UPDATE", ids)
	if err != nil {
		return fmt.Errorf("lock courses: %w", err)
	}
	defer rows.Close()
	found := 0
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan locked course: %w", err)
		}
		found++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("lock courses: %w", err)
	}
	if found != len(ids) {
		return ErrNotFound
	}
	return nil
}

func (r *coursePostgresRepository) UpdateQuota(ctx context.Context, id, quota, version int) (model.Course, error) {
	course, err := scanCourse(r.db.QueryRow(ctx,
		"UPDATE courses SET quota=$1, version=version+1 WHERE id=$2 AND version=$3 AND $1>=enrolled_count RETURNING "+courseColumns,
		quota, id, version))
	if err == nil {
		return course, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Course{}, fmt.Errorf("update quota: %w", err)
	}
	current, findErr := r.FindByID(ctx, id)
	if findErr != nil {
		return model.Course{}, findErr
	}
	if current.Version != version {
		return model.Course{}, ErrVersionConflict
	}
	return model.Course{}, ErrQuotaBelowFilled
}

func scanCourse(row pgx.Row) (model.Course, error) {
	var course model.Course
	err := row.Scan(&course.ID, &course.Code, &course.Name, &course.Credits, &course.Quota,
		&course.EnrolledCount, &course.Available, &course.Version, &course.CreatedAt)
	if err != nil {
		return model.Course{}, err
	}
	return course, nil
}
