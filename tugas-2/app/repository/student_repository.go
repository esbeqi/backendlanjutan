package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"api-students/app/model"
)

var (
	ErrNotFound  = errors.New("data tidak ditemukan")
	ErrDuplicate = errors.New("data sudah ada")
)

type StudentRepository interface {
	FindAll(ctx context.Context, q model.ListQuery) ([]model.Student, int, error)
	FindByID(ctx context.Context, id int) (model.Student, error)
	Create(ctx context.Context, s model.Student) (model.Student, error)
	Update(ctx context.Context, s model.Student) (model.Student, error)
	Delete(ctx context.Context, id int) error
}

type studentPostgresRepository struct {
	pool *pgxpool.Pool
}

func NewStudentRepository(pool *pgxpool.Pool) StudentRepository {
	return &studentPostgresRepository{
		pool: pool,
	}
}

func (r *studentPostgresRepository) FindByID(ctx context.Context, id int) (model.Student, error) {
	var s model.Student

	err := r.pool.QueryRow(ctx, `
		SELECT id, nim, name, grade, is_active, created_at
		FROM students
		WHERE id = $1
	`, id).Scan(
		&s.ID,
		&s.NIM,
		&s.Name,
		&s.Grade,
		&s.IsActive,
		&s.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Student{}, ErrNotFound
		}
		return model.Student{}, err
	}

	return s, nil
}

func (r *studentPostgresRepository) Delete(ctx context.Context, id int) error {

	cmd, err := r.pool.Exec(ctx,
		`DELETE FROM students WHERE id=$1`,
		id,
	)

	if err != nil {
		return err
	}

	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *studentPostgresRepository) Create(ctx context.Context, s model.Student) (model.Student, error) {

	err := r.pool.QueryRow(ctx, `
		INSERT INTO students
		(nim,name,grade,is_active)
		VALUES ($1,$2,$3,$4)
		RETURNING id,created_at
	`,
		s.NIM,
		s.Name,
		s.Grade,
		s.IsActive,
	).Scan(
		&s.ID,
		&s.CreatedAt,
	)

	if err != nil {

		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" {

			return model.Student{}, ErrDuplicate
		}

		return model.Student{}, err
	}

	return s, nil
}

func (r *studentPostgresRepository) Update(ctx context.Context, s model.Student) (model.Student, error) {

	cmd, err := r.pool.Exec(ctx, `
		UPDATE students
		SET
			nim=$1,
			name=$2,
			grade=$3,
			is_active=$4
		WHERE id=$5
	`,
		s.NIM,
		s.Name,
		s.Grade,
		s.IsActive,
		s.ID,
	)

	if err != nil {

		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" {

			return model.Student{}, ErrDuplicate
		}

		return model.Student{}, err
	}

	if cmd.RowsAffected() == 0 {
		return model.Student{}, ErrNotFound
	}

	return r.FindByID(ctx, s.ID)
}

func (r *studentPostgresRepository) FindAll(ctx context.Context, q model.ListQuery) ([]model.Student, int, error) {

	allowedSort := map[string]string{
		"id":         "id",
		"nim":        "nim",
		"name":       "name",
		"grade":      "grade",
		"created_at": "created_at",
	}

	sortBy := allowedSort[q.Sort]
	if sortBy == "" {
		sortBy = "id"
	}

	order := "ASC"
	if strings.ToLower(q.Order) == "desc" {
		order = "DESC"
	}

	where := []string{}
	args := []any{}
	arg := 1

	if q.Search != "" {
		where = append(where, fmt.Sprintf("name ILIKE $%d", arg))
		args = append(args, "%"+q.Search+"%")
		arg++
	}

	if q.IsActive != nil {
		where = append(where, fmt.Sprintf("is_active = $%d", arg))
		args = append(args, *q.IsActive)
		arg++
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	var total int

	countSQL := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM students
		%s
	`, whereSQL)

	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, q.Limit, q.Offset())

	sql := fmt.Sprintf(`
		SELECT
			id,
			nim,
			name,
			grade,
			is_active,
			created_at
		FROM students
		%s
		ORDER BY %s %s
		LIMIT $%d
		OFFSET $%d
	`,
		whereSQL,
		sortBy,
		order,
		arg,
		arg+1,
	)

	rows, err := r.pool.Query(ctx, sql, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var students []model.Student

	for rows.Next() {

		var s model.Student

		if err := rows.Scan(
			&s.ID,
			&s.NIM,
			&s.Name,
			&s.Grade,
			&s.IsActive,
			&s.CreatedAt,
		); err != nil {
			return nil, 0, err
		}

		students = append(students, s)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return students, total, nil
}
