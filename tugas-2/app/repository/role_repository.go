package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RoleRepository interface {
	FindPermissions(ctx context.Context) (map[string][]string, error)
}

type rolePostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRoleRepository(pool *pgxpool.Pool) RoleRepository {
	return &rolePostgresRepository{pool: pool}
}

func (r *rolePostgresRepository) FindPermissions(
	ctx context.Context,
) (map[string][]string, error) {

	rows, err := r.pool.Query(ctx, `
		SELECT role_name, permission_name
		FROM role_permissions
		ORDER BY role_name, permission_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)

	for rows.Next() {
		var role string
		var permission string

		if err := rows.Scan(&role, &permission); err != nil {
			return nil, err
		}

		result[role] = append(result[role], permission)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
