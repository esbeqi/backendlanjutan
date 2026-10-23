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
	WithTx(tx DBTX) StudentRepository
	FindAll(ctx context.Context, q model.ListQuery) ([]model.Student, int, error)
	FindAllCursor(ctx context.Context, q model.CursorQuery) ([]model.Student, *model.Cursor, error)
	FindByID(ctx context.Context, id int) (model.Student, error)
	Create(ctx context.Context, s model.Student) (model.Student, error)
	Update(ctx context.Context, s model.Student) (model.Student, error)
	Delete(ctx context.Context, id int) error
}

type studentPostgresRepository struct {
	db DBTX
}

func NewStudentRepository(pool *pgxpool.Pool) StudentRepository {
	return &studentPostgresRepository{
		db: pool,
	}
}

func (r *studentPostgresRepository) WithTx(tx DBTX) StudentRepository {
	return &studentPostgresRepository{db: tx}
}

// FindByID mengambil satu data mahasiswa berdasarkan ID.
func (r *studentPostgresRepository) FindByID(
	ctx context.Context,
	id int,
) (model.Student, error) {

	var s model.Student

	err := r.db.QueryRow(ctx, `
		SELECT
			id,
			nim,
			name,
			grade,
			is_active,
			created_at,
			owner_id
		FROM students
		WHERE id = $1
	`, id).Scan(
		&s.ID,
		&s.NIM,
		&s.Name,
		&s.Grade,
		&s.IsActive,
		&s.CreatedAt,
		&s.OwnerID,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Student{}, ErrNotFound
		}

		return model.Student{}, err
	}

	return s, nil
}

// Delete menghapus mahasiswa berdasarkan ID.
func (r *studentPostgresRepository) Delete(
	ctx context.Context,
	id int,
) error {

	cmd, err := r.db.Exec(
		ctx,
		`DELETE FROM students WHERE id = $1`,
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

// Create menambahkan data mahasiswa baru.
func (r *studentPostgresRepository) Create(
	ctx context.Context,
	s model.Student,
) (model.Student, error) {

	err := r.db.QueryRow(ctx, `
		INSERT INTO students
			(nim, name, grade, is_active, owner_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`,
		s.NIM,
		s.Name,
		s.Grade,
		s.IsActive,
		s.OwnerID,
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

// Update memperbarui data mahasiswa.
func (r *studentPostgresRepository) Update(
	ctx context.Context,
	s model.Student,
) (model.Student, error) {

	cmd, err := r.db.Exec(ctx, `
		UPDATE students
		SET
			nim = $1,
			name = $2,
			grade = $3,
			is_active = $4
		WHERE id = $5
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

// FindAll digunakan untuk pagination berbasis offset.
func (r *studentPostgresRepository) FindAll(
	ctx context.Context,
	q model.ListQuery,
) ([]model.Student, int, error) {

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
		where = append(
			where,
			fmt.Sprintf("name ILIKE $%d", arg),
		)

		args = append(
			args,
			"%"+q.Search+"%",
		)

		arg++
	}

	if q.IsActive != nil {
		where = append(
			where,
			fmt.Sprintf("is_active = $%d", arg),
		)

		args = append(
			args,
			*q.IsActive,
		)

		arg++
	}

	whereSQL := ""

	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(
			where,
			" AND ",
		)
	}

	var total int

	countSQL := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM students
		%s
	`, whereSQL)

	if err := r.db.QueryRow(
		ctx,
		countSQL,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	queryArgs := append([]any{}, args...)

	queryArgs = append(
		queryArgs,
		q.Limit,
		q.Offset(),
	)

	sql := fmt.Sprintf(`
		SELECT
			id,
			nim,
			name,
			grade,
			is_active,
			created_at,
			owner_id
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

	rows, err := r.db.Query(
		ctx,
		sql,
		queryArgs...,
	)

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
			&s.OwnerID,
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

// FindAllCursor digunakan untuk pagination berbasis cursor.
//
// Urutan:
// created_at ASC, id ASC
//
// created_at menjadi pengurut utama.
// id menjadi tie-breaker sehingga pasangan
// (created_at, id) menentukan posisi data secara unik.
func (r *studentPostgresRepository) FindAllCursor(
	ctx context.Context,
	q model.CursorQuery,
) ([]model.Student, *model.Cursor, error) {

	where := []string{}
	args := []any{}
	arg := 1

	if q.Search != "" {
		where = append(
			where,
			fmt.Sprintf("name ILIKE $%d", arg),
		)

		args = append(
			args,
			"%"+q.Search+"%",
		)

		arg++
	}

	if q.IsActive != nil {
		where = append(
			where,
			fmt.Sprintf("is_active = $%d", arg),
		)

		args = append(
			args,
			*q.IsActive,
		)

		arg++
	}

	// Jika cursor diberikan, ambil data
	// setelah pasangan (created_at, id).
	if q.CursorCreatedAt != nil &&
		q.CursorID != nil {

		where = append(
			where,
			fmt.Sprintf(
				"(created_at, id) > ($%d, $%d)",
				arg,
				arg+1,
			),
		)

		args = append(
			args,
			*q.CursorCreatedAt,
			*q.CursorID,
		)

		arg += 2
	}

	whereSQL := ""

	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(
			where,
			" AND ",
		)
	}

	// Ambil satu data tambahan untuk mengetahui
	// apakah masih ada halaman berikutnya.
	queryArgs := append([]any{}, args...)

	queryArgs = append(
		queryArgs,
		q.Limit+1,
	)

	sql := fmt.Sprintf(`
		SELECT
			id,
			nim,
			name,
			grade,
			is_active,
			created_at,
			owner_id
		FROM students
		%s
		ORDER BY created_at ASC, id ASC
		LIMIT $%d
	`,
		whereSQL,
		arg,
	)

	rows, err := r.db.Query(
		ctx,
		sql,
		queryArgs...,
	)

	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	students := make(
		[]model.Student,
		0,
		q.Limit+1,
	)

	for rows.Next() {
		var s model.Student

		if err := rows.Scan(
			&s.ID,
			&s.NIM,
			&s.Name,
			&s.Grade,
			&s.IsActive,
			&s.CreatedAt,
			&s.OwnerID,
		); err != nil {
			return nil, nil, err
		}

		students = append(
			students,
			s,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var nextCursor *model.Cursor

	// Jika jumlah data melebihi limit,
	// berarti masih ada halaman berikutnya.
	if len(students) > q.Limit {

		students = students[:q.Limit]

		last := students[len(students)-1]

		nextCursor = &model.Cursor{
			CreatedAt: last.CreatedAt,
			ID:        last.ID,
		}
	}

	return students, nextCursor, nil
}
