package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
	"github.com/hibiken/asynq"
)

type TaskEnqueuer interface {
	EnqueueContext(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}
type Relay struct {
	tx     *repository.TxManager
	outbox repository.OutboxRepository
	client TaskEnqueuer
	logger *slog.Logger
}

func NewRelay(tx *repository.TxManager, o repository.OutboxRepository, c TaskEnqueuer, l *slog.Logger) *Relay {
	return &Relay{tx, o, c, l}
}
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Publish(ctx); err != nil {
				r.logger.Error("relay_delayed", "published", 0, "error", err.Error())
			}
		}
	}
}
func (r *Relay) Publish(ctx context.Context) error {
	return r.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := r.outbox.WithTx(tx)
		events, err := repo.FetchUnpublished(ctx, 50)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(events))
		for _, ev := range events {
			var payload model.CourseEvent
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				return err
			}
			for _, recipient := range payload.Recipients {
				body, err := json.Marshal(model.NotificationTask{OutboxID: ev.ID, EventType: ev.EventType, Recipient: recipient})
				if err != nil {
					return err
				}
				task := asynq.NewTask(model.TaskSendNotification, body)
				_, err = r.client.EnqueueContext(ctx, task, asynq.Queue(model.QueueNotifications), asynq.TaskID(notificationTaskID(ev.ID, recipient.UserID)), asynq.MaxRetry(5))
				if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
					return err
				}
			}
			ids = append(ids, ev.ID)
		}
		if err := repo.MarkPublished(ctx, ids); err != nil {
			return err
		}
		if len(ids) > 0 {
			r.logger.Info("relay_done", "published", len(ids))
		}
		return nil
	})
}

func notificationTaskID(outboxID int64, userID int) string {
	return fmt.Sprintf("outbox-%d-user-%d", outboxID, userID)
}

type NotificationWorker struct {
	tx            *repository.TxManager
	notifications repository.NotificationRepository
	mailer        *helper.Mailer
	logger        *slog.Logger
}

func NewNotificationWorker(tx *repository.TxManager, n repository.NotificationRepository, m *helper.Mailer, l *slog.Logger) *NotificationWorker {
	return &NotificationWorker{tx, n, m, l}
}
func (w *NotificationWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	if task.Type() != model.TaskSendNotification {
		return fmt.Errorf("unknown task type: %s", task.Type())
	}
	var item model.NotificationTask
	if err := json.Unmarshal(task.Payload(), &item); err != nil {
		return asynq.SkipRetry
	}
	claimed := false
	if err := w.tx.WithTx(ctx, func(tx repository.DBTX) error {
		var e error
		claimed, e = w.notifications.WithTx(tx).Claim(ctx, item.OutboxID, item.Recipient)
		return e
	}); err != nil {
		return err
	}
	if !claimed {
		alreadySent, err := w.notifications.IsSent(ctx, item.OutboxID, item.Recipient.UserID)
		if err != nil {
			return err
		}
		failed, err := w.notifications.IsFailed(ctx, item.OutboxID, item.Recipient.UserID)
		if err != nil {
			return err
		}
		if failed {
			return nil
		}
		if !alreadySent {
			return errors.New("notification penerima sedang diproses oleh task lain")
		}
		w.logger.Info("notification_processed", "outbox_id", item.OutboxID, "event", item.EventType, "retry", retryNumber(ctx), "already_sent", true)
		return nil
	}
	var sendErr error
	err := w.tx.WithTxOnce(ctx, func(tx repository.DBTX) error {
		if err := w.mailer.Send(item.Recipient.Email, item.Recipient.Subject, item.Recipient.Body); err != nil {
			return err
		}
		return w.notifications.WithTx(tx).MarkSent(ctx, item.OutboxID, item.Recipient.UserID)
	})
	if err != nil {
		sendErr = err
		if errors.Is(sendErr, helper.ErrPermanentFailure) {
			if markErr := w.tx.WithTxOnce(ctx, func(tx repository.DBTX) error {
				return w.notifications.WithTx(tx).MarkFailed(ctx, item.OutboxID, item.Recipient.UserID, sendErr.Error())
			}); markErr != nil {
				return errors.Join(sendErr, markErr)
			}
			w.logger.Error("task_failed", "type", task.Type(), "attempt", fmt.Sprintf("%d/6", retryNumber(ctx)+1), "error", sendErr.Error())
			return asynq.SkipRetry
		}
		if releaseErr := w.tx.WithTxOnce(ctx, func(tx repository.DBTX) error {
			return w.notifications.WithTx(tx).Release(ctx, item.OutboxID, item.Recipient.UserID)
		}); releaseErr != nil {
			return errors.Join(sendErr, releaseErr)
		}
		w.logger.Error("task_failed", "type", task.Type(), "attempt", fmt.Sprintf("%d/6", retryNumber(ctx)+1), "error", sendErr.Error())
		return sendErr
	}
	w.logger.Info("notification_processed", "outbox_id", item.OutboxID, "event", item.EventType, "retry", retryNumber(ctx), "already_sent", false)
	return nil
}
func retryNumber(ctx context.Context) int { n, _ := asynq.GetRetryCount(ctx); return n }

func QueueMonitor(client *asynq.Inspector, queue string) (*model.QueueInfo, error) {
	info, err := client.GetQueueInfo(queue)
	if err != nil {
		return nil, err
	}
	return &model.QueueInfo{Name: queue, Pending: info.Pending, Active: info.Active, Scheduled: info.Scheduled, Retry: info.Retry, Archived: info.Archived, Completed: info.Completed, Processed: info.Processed, Failed: info.Failed}, nil
}
