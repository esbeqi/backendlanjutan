package repository

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const rollbackTimeout = 5 * time.Second
const transactionMaxAttempts = 3

// TxManager menjadi satu-satunya komponen aplikasi yang membuka transaksi.
type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithTx mengulang seluruh callback hanya untuk deadlock atau serialization
// failure. Callbacks harus berisi operasi database yang aman untuk diulang.
func (m *TxManager) WithTx(ctx context.Context, fn func(DBTX) error) error {
	var err error
	for attempt := 1; attempt <= transactionMaxAttempts; attempt++ {
		err = m.runOnce(ctx, fn)
		if !isRetryableTransactionError(err) {
			return err
		}
		wait := time.Duration(attempt*20+rand.IntN(30)) * time.Millisecond
		select {
		case <-ctx.Done():
			return err
		case <-time.After(wait):
		}
	}
	return err
}

// WithTxOnce tidak mengulang callback. Gunakan untuk callback dengan side
// effect eksternal, misalnya mengirim email, yang tidak dapat di-rollback.
func (m *TxManager) WithTxOnce(ctx context.Context, fn func(DBTX) error) error {
	return m.runOnce(ctx, fn)
}

func (m *TxManager) runOnce(ctx context.Context, fn func(DBTX) error) (err error) {
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

func isRetryableTransactionError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "40001" || pgErr.Code == "40P01"
}
