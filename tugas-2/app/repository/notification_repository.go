package repository

import (
	"context"
	"errors"
	"fmt"

	"api-students/app/model"
	"github.com/jackc/pgx/v5"
)

type NotificationRepository interface {
	WithTx(tx DBTX) NotificationRepository
	Claim(ctx context.Context, outboxID int64, recipient model.NotificationRecipient) (bool, error)
	MarkSent(ctx context.Context, outboxID int64, userID int) error
	Release(ctx context.Context, outboxID int64, userID int) error
	IsSent(ctx context.Context, outboxID int64, userID int) (bool, error)
	MarkFailed(ctx context.Context, outboxID int64, userID int, message string) error
	IsFailed(ctx context.Context, outboxID int64, userID int) (bool, error)
	FindByUser(ctx context.Context, userID, limit int) ([]model.Notification, error)
	Count(ctx context.Context) (int, error)
}

type notificationPostgresRepository struct{ db DBTX }

func NewNotificationRepository(db DBTX) NotificationRepository {
	return &notificationPostgresRepository{db: db}
}

func (r *notificationPostgresRepository) WithTx(tx DBTX) NotificationRepository {
	return &notificationPostgresRepository{db: tx}
}

func (r *notificationPostgresRepository) Claim(ctx context.Context, outboxID int64, recipient model.NotificationRecipient) (bool, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO notifications(outbox_id,user_id,channel,subject,body,status,processing_until)
		VALUES($1,$2,'email',$3,$4,'processing',NOW()+INTERVAL '5 minutes')
		ON CONFLICT (outbox_id,user_id) DO UPDATE SET status='processing',processing_until=NOW()+INTERVAL '5 minutes',subject=EXCLUDED.subject,body=EXCLUDED.body
		WHERE notifications.status = 'pending' OR (notifications.status='processing' AND notifications.processing_until < NOW())
		RETURNING id`, outboxID, recipient.UserID, recipient.Subject, recipient.Body).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("mencatat notification claim: %w", err)
	}
	return true, nil
}

func (r *notificationPostgresRepository) Release(ctx context.Context, outboxID int64, userID int) error {
	_, err := r.db.Exec(ctx, `UPDATE notifications SET status='pending',processing_until=NULL WHERE outbox_id=$1 AND user_id=$2 AND status='processing'`, outboxID, userID)
	if err != nil {
		return fmt.Errorf("melepas notification untuk retry: %w", err)
	}
	return nil
}

func (r *notificationPostgresRepository) IsSent(ctx context.Context, outboxID int64, userID int) (bool, error) {
	var sent bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM notifications WHERE outbox_id=$1 AND user_id=$2 AND status='sent')`, outboxID, userID).Scan(&sent)
	if err != nil {
		return false, fmt.Errorf("memeriksa status notification: %w", err)
	}
	return sent, nil
}

func (r *notificationPostgresRepository) MarkFailed(ctx context.Context, outboxID int64, userID int, message string) error {
	_, err := r.db.Exec(ctx, `UPDATE notifications SET status='failed',processing_until=NULL,last_error=$3 WHERE outbox_id=$1 AND user_id=$2`, outboxID, userID, message)
	if err != nil {
		return fmt.Errorf("menandai notification gagal permanen: %w", err)
	}
	return nil
}
func (r *notificationPostgresRepository) IsFailed(ctx context.Context, outboxID int64, userID int) (bool, error) {
	var failed bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM notifications WHERE outbox_id=$1 AND user_id=$2 AND status='failed')`, outboxID, userID).Scan(&failed)
	if err != nil {
		return false, fmt.Errorf("memeriksa kegagalan notification: %w", err)
	}
	return failed, nil
}

func (r *notificationPostgresRepository) MarkSent(ctx context.Context, outboxID int64, userID int) error {
	_, err := r.db.Exec(ctx, `UPDATE notifications SET status='sent',sent_at=NOW(),processing_until=NULL WHERE outbox_id=$1 AND user_id=$2`, outboxID, userID)
	if err != nil {
		return fmt.Errorf("menandai notification terkirim: %w", err)
	}
	return nil
}

func (r *notificationPostgresRepository) FindByUser(ctx context.Context, userID, limit int) ([]model.Notification, error) {
	rows, err := r.db.Query(ctx, `
		SELECT n.id,n.outbox_id,n.user_id,n.channel,n.subject,n.body,n.sent_at
		FROM notifications n
		WHERE n.user_id=$1
		ORDER BY n.outbox_id DESC,n.user_id
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("mengambil notifications user: %w", err)
	}
	defer rows.Close()
	result := make([]model.Notification, 0)
	for rows.Next() {
		var notification model.Notification
		if err := rows.Scan(&notification.ID, &notification.OutboxID, &notification.UserID, &notification.Channel, &notification.Subject, &notification.Body, &notification.SentAt); err != nil {
			return nil, fmt.Errorf("membaca notification: %w", err)
		}
		result = append(result, notification)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mengiterasi notifications: %w", err)
	}
	return result, nil
}

func (r *notificationPostgresRepository) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM notifications`).Scan(&count); err != nil {
		return 0, fmt.Errorf("menghitung notifications: %w", err)
	}
	return count, nil
}
