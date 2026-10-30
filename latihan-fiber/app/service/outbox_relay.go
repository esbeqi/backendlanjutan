package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"latihan-fiber/app/model"
	"latihan-fiber/app/repository"
)

type TaskEnqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// OutboxRelay memindahkan event PostgreSQL ke Redis. API tidak bergantung
// pada Redis, sehingga gangguan broker hanya menunda notifikasi.
type OutboxRelay struct {
	outbox   repository.OutboxRepository
	tx       *repository.TxManager
	queue    TaskEnqueuer
	logger   *slog.Logger
	interval time.Duration
	batch    int
}

func NewOutboxRelay(outbox repository.OutboxRepository, tx *repository.TxManager, queue TaskEnqueuer, logger *slog.Logger, interval time.Duration, batch int) *OutboxRelay {
	return &OutboxRelay{outbox: outbox, tx: tx, queue: queue, logger: logger, interval: interval, batch: batch}
}

func (r *OutboxRelay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		n, err := r.RelayOnce(ctx)
		if err != nil {
			r.logger.Warn("relay_delayed", slog.Int("published", n), slog.String("error", err.Error()))
		} else if n > 0 {
			r.logger.Info("relay_done", slog.Int("published", n))
		}
		if err == nil && n == r.batch && ctx.Err() == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *OutboxRelay) RelayOnce(ctx context.Context) (int, error) {
	var published int
	var enqueueErr error
	err := r.tx.WithTxOnce(ctx, func(tx repository.DBTX) error {
		repo := r.outbox.WithTx(tx)
		events, err := repo.FetchUnpublished(ctx, r.batch)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(events))
		for _, e := range events {
			// Berhenti pada kegagalan pertama. Event yang belum berhasil
			// dipublish tetap belum ditandai dan dicoba pada putaran berikutnya.
			if err := r.publish(ctx, e); err != nil {
				enqueueErr = err
				break
			}
			ids = append(ids, e.ID)
		}
		if err := repo.MarkPublished(ctx, ids); err != nil {
			return err
		}
		published = len(ids)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return published, enqueueErr
}

func (r *OutboxRelay) publish(ctx context.Context, e model.OutboxEvent) error {
	job := model.NotificationJob{OutboxID: e.ID, EventType: e.EventType, Payload: e.Payload}
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode outbox %d: %w", e.ID, err)
	}
	task := asynq.NewTask(model.TaskSendNotification, payload,
		// TaskID menutup celah enqueue ulang bila relay mati sebelum commit.
		asynq.TaskID(fmt.Sprintf("outbox-%d", e.ID)),
		asynq.Queue(model.QueueNotifications),
		asynq.MaxRetry(5), asynq.Timeout(30*time.Second),
		// Retention mempertahankan TaskID agar deduplikasi berlaku selama 24 jam.
		asynq.Retention(24*time.Hour))
	_, err = r.queue.EnqueueContext(ctx, task)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("enqueue outbox %d: %w", e.ID, err)
	}
	return nil
}
