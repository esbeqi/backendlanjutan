package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"api-students/app/model"
)

type OutboxRepository interface {
	WithTx(tx DBTX) OutboxRepository
	Insert(ctx context.Context, eventType string, event any) (int64, error)
	FetchUnpublished(ctx context.Context, limit int) ([]model.OutboxEvent, error)
	MarkPublished(ctx context.Context, ids []int64) error
	Stats(ctx context.Context) (pending, published int, oldest time.Duration, err error)
}

type outboxPostgresRepository struct{ db DBTX }

func NewOutboxRepository(db DBTX) OutboxRepository { return &outboxPostgresRepository{db: db} }

func (r *outboxPostgresRepository) WithTx(tx DBTX) OutboxRepository {
	return &outboxPostgresRepository{db: tx}
}

func (r *outboxPostgresRepository) Insert(ctx context.Context, eventType string, event any) (int64, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return 0, fmt.Errorf("menyusun payload outbox: %w", err)
	}
	var id int64
	if err := r.db.QueryRow(ctx, `INSERT INTO outbox(event_type,payload) VALUES($1,$2) RETURNING id`, eventType, payload).Scan(&id); err != nil {
		return 0, fmt.Errorf("mencatat outbox: %w", err)
	}
	return id, nil
}

func (r *outboxPostgresRepository) FetchUnpublished(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id,event_type,payload,created_at
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY id
		LIMIT $1
		FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, fmt.Errorf("mengambil outbox belum publish: %w", err)
	}
	defer rows.Close()
	result := make([]model.OutboxEvent, 0)
	for rows.Next() {
		var event model.OutboxEvent
		if err := rows.Scan(&event.ID, &event.EventType, &event.Payload, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("membaca baris outbox: %w", err)
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mengiterasi outbox: %w", err)
	}
	return result, nil
}

func (r *outboxPostgresRepository) MarkPublished(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := r.db.Exec(ctx, `UPDATE outbox SET published_at=NOW() WHERE id=ANY($1)`, ids); err != nil {
		return fmt.Errorf("menandai outbox published: %w", err)
	}
	return nil
}

func (r *outboxPostgresRepository) Stats(ctx context.Context) (int, int, time.Duration, error) {
	var pending, published int
	var seconds float64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE published_at IS NULL),
		       COUNT(*) FILTER (WHERE published_at IS NOT NULL),
		       COALESCE(EXTRACT(EPOCH FROM NOW()-MIN(created_at) FILTER (WHERE published_at IS NULL)),0)
		FROM outbox`).Scan(&pending, &published, &seconds)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("menghitung outbox: %w", err)
	}
	return pending, published, time.Duration(seconds * float64(time.Second)), nil
}
