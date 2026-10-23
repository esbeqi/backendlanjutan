package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type supervisorLockDBTX struct {
	existingIDs map[int]bool
	lockedIDs   []int
}

func (db *supervisorLockDBTX) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}

func (db *supervisorLockDBTX) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (db *supervisorLockDBTX) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	id := args[0].(int)
	db.lockedIDs = append(db.lockedIDs, id)
	return supervisorLockRow{id: id, exists: db.existingIDs[id]}
}

type supervisorLockRow struct {
	id     int
	exists bool
}

func (r supervisorLockRow) Scan(dest ...any) error {
	if !r.exists {
		return pgx.ErrNoRows
	}
	*dest[0].(*int) = r.id
	return nil
}

func TestLockSupervisorsUsesAscendingUniqueIDs(t *testing.T) {
	db := &supervisorLockDBTX{existingIDs: map[int]bool{2: true, 8: true}}
	repo := NewSupervisorRepository(db)
	if err := repo.LockSupervisors(context.Background(), 8, 2, 8); err != nil {
		t.Fatalf("LockSupervisors: %v", err)
	}
	if len(db.lockedIDs) != 2 || db.lockedIDs[0] != 2 || db.lockedIDs[1] != 8 {
		t.Fatalf("locked IDs = %v, want [2 8]", db.lockedIDs)
	}
}

func TestLockSupervisorsReturnsNotFound(t *testing.T) {
	db := &supervisorLockDBTX{existingIDs: map[int]bool{2: true}}
	repo := NewSupervisorRepository(db)
	if err := repo.LockSupervisors(context.Background(), 8, 2); !errors.Is(err, ErrSupervisorNotFound) {
		t.Fatalf("error = %v, want ErrSupervisorNotFound", err)
	}
	if len(db.lockedIDs) != 2 || db.lockedIDs[0] != 2 || db.lockedIDs[1] != 8 {
		t.Fatalf("lock attempts = %v, want [2 8]", db.lockedIDs)
	}
}
