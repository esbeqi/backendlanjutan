package model

import "time"

const (
	EventCourseQuotaChanged = "course.quota_changed"
	EventEnrollmentCreated  = "enrollment.created"
	EventEnrollmentDropped  = "enrollment.dropped"
	TaskSendNotification    = "notification:send"
	QueueNotifications      = "notifications"
)

type Course struct {
	ID            int       `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	Quota         int       `json:"quota"`
	EnrolledCount int       `json:"enrolled_count"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
}

type PatchCourseRequest struct {
	Quota   int `json:"quota"`
	Version int `json:"version"`
}

type CourseEvent struct {
	CourseID   int                     `json:"course_id"`
	CourseCode string                  `json:"course_code"`
	CourseName string                  `json:"course_name"`
	Quota      int                     `json:"quota,omitempty"`
	Recipients []NotificationRecipient `json:"recipients"`
}

type NotificationRecipient struct {
	UserID  int    `json:"user_id"`
	Email   string `json:"email"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type NotificationTask struct {
	OutboxID  int64                 `json:"outbox_id"`
	EventType string                `json:"event_type"`
	Recipient NotificationRecipient `json:"recipient"`
}

type OutboxEvent struct {
	ID        int64
	EventType string
	Payload   []byte
	CreatedAt time.Time
}

type Notification struct {
	ID       int64     `json:"id"`
	OutboxID int64     `json:"outbox_id"`
	UserID   int       `json:"user_id"`
	Channel  string    `json:"channel"`
	Subject  string    `json:"subject"`
	Body     string    `json:"body"`
	SentAt   time.Time `json:"sent_at"`
}

type QueueStats struct {
	OutboxPending     int        `json:"outbox_pending"`
	OutboxPublished   int        `json:"outbox_published"`
	OldestPendingAge  string     `json:"oldest_pending_age"`
	NotificationsSent int        `json:"notifications_sent"`
	Queue             *QueueInfo `json:"queue"`
	QueueError        string     `json:"queue_error,omitempty"`
}

type QueueInfo struct {
	Name      string `json:"name"`
	Pending   int    `json:"pending"`
	Active    int    `json:"active"`
	Scheduled int    `json:"scheduled"`
	Retry     int    `json:"retry"`
	Archived  int    `json:"archived"`
	Completed int    `json:"completed"`
	Processed int    `json:"processed_today"`
	Failed    int    `json:"failed_today"`
}
