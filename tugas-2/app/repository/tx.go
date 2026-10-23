package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const rollbackTimeout = 5 * time.Second

// TxManager menjadi satu-satunya komponen aplikasi yang membuka transaksi.
type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithTx menjalankan callback satu kali dalam satu transaksi.
// Seluruh repository di callback harus dibuat dengan tx yang sama.
// Hindari side effect eksternal di callback karena tidak dapat di-rollback
// bila callback atau commit gagal.
func (m *TxManager) WithTx(ctx context.Context, fn func(DBTX) error) (err error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		// Gunakan context yang tetap aktif untuk rollback walau request dibatalkan.
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()

		if rollbackErr := tx.Rollback(rollbackCtx); rollbackErr != nil &&
			!errors.Is(rollbackErr, pgx.ErrTxClosed) {
			rollbackErr = fmt.Errorf("rollback transaction: %w", rollbackErr)
			if err == nil {
				err = rollbackErr
			} else {
				err = errors.Join(err, rollbackErr)
			}
		}
	}()

	if err = fn(tx); err != nil {
		return fmt.Errorf("transaction callback: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
