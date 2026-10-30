// Command mod11ops runs the Modul 11 operational scenarios against the local
// API and PostgreSQL. It never prints access tokens or configuration secrets.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"latihan-fiber/app/model"
	"latihan-fiber/config"
	"latihan-fiber/database"
	"latihan-fiber/helper"
)

const testPassword = "Modul11-Test-2026"

type runner struct {
	base   string
	client *http.Client
	pool   *pgxpool.Pool
	ctx    context.Context
}

type envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Code    string          `json:"code"`
	Data    json.RawMessage `json:"data"`
}

func main() {
	mode := flag.String("scenario", "", "register | c1 | c2down | c2up | reset | c3 | c4before | c4 | c5enroll | c5verify | c6enroll | c6verify | snapshot")
	username := flag.String("user", "mhsuji", "test username for snapshot")
	resetPath := flag.String("reset-sql", filepath.Join("test-evidence", "C.3", "reset11.sql"), "path to the saved Modul 11 reset SQL")
	flag.Parse()
	if *mode == "" {
		fmt.Fprintln(os.Stderr, "scenario is required")
		os.Exit(2)
	}
	config.LoadEnv()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool, err := database.NewPool(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "PostgreSQL connection failed (details suppressed)")
		os.Exit(1)
	}
	defer pool.Close()
	r := &runner{base: "http://localhost:" + config.GetEnv("APP_PORT", "3000"), client: &http.Client{Timeout: 5 * time.Second}, pool: pool, ctx: ctx}
	if *mode == "snapshot" {
		if err := r.printAccountEvidence("snapshot", *username, -1); err != nil {
			fmt.Fprintln(os.Stderr, "snapshot error:", err)
			os.Exit(1)
		}
		return
	}
	if err := r.run(*mode, *resetPath); err != nil {
		fmt.Fprintln(os.Stderr, "scenario error:", err)
		os.Exit(1)
	}
}

func (r *runner) run(mode, resetPath string) error {
	switch mode {
	case "register":
		for _, account := range []struct{ name, email string }{
			{"mhsuji", "mhsuji@student.unair.ac.id"},
			{"salahalamat", "salah@kampus.invalid"},
		} {
			status, body, err := r.request(http.MethodPost, "/api/v1/auth/register", "", map[string]string{"username": account.name, "email": account.email, "password": testPassword})
			if err != nil {
				return err
			}
			fmt.Printf("register %s status=%d message=%s\n", account.name, status, messageOf(body))
			if status == http.StatusConflict {
				if _, err := r.login(account.name); err != nil {
					return fmt.Errorf("existing account %s cannot log in", account.name)
				}
			}
		}
		for _, name := range []string{"mhsuji", "salahalamat"} {
			if _, err := r.login(name); err != nil {
				return fmt.Errorf("login %s failed", name)
			}
			fmt.Printf("login %s status=200 token=not-printed\n", name)
		}
		return nil
	case "c1":
		return r.c1()
	case "c2down":
		return r.c2down()
	case "c2up":
		return r.c2up()
	case "reset":
		data, err := os.ReadFile(resetPath)
		if err != nil {
			return fmt.Errorf("read saved reset SQL: %w", err)
		}
		if _, err := r.pool.Exec(r.ctx, string(data)); err != nil {
			return fmt.Errorf("execute reset SQL: %w", err)
		}
		fmt.Println("reset11.sql applied to accounts mhsuji and salahalamat")
		return nil
	case "c3":
		return r.c3()
	case "c4before":
		q, err := r.queueStats()
		if err != nil {
			return err
		}
		fmt.Printf("C4 baseline archived=%d retry=%d pending=%d\n", queueArchived(q), queueRetry(q), q.OutboxPending)
		return nil
	case "c4":
		return r.c4()
	case "c5enroll":
		return r.c5enroll()
	case "c5verify":
		return r.c5verify()
	case "c6enroll":
		return r.c6enroll()
	case "c6verify":
		return r.c6verify()
	default:
		return fmt.Errorf("unknown scenario %q", mode)
	}
}

func (r *runner) c1() error {
	token, err := r.login("mhsuji")
	if err != nil {
		return err
	}
	for _, step := range []struct {
		method, path string
		payload      any
	}{
		{http.MethodPost, "/api/v1/courses/2/enroll", map[string]any{}},
		{http.MethodPost, "/api/v1/courses/2/enroll", map[string]any{}},
		{http.MethodDelete, "/api/v1/courses/2/enroll", map[string]any{}},
		{http.MethodPost, "/api/v1/courses/2/enroll", map[string]any{}},
		{http.MethodPost, "/api/v1/enrollments/swap", map[string]int{"from_course_id": 2, "to_course_id": 4}},
	} {
		status, body, elapsed, callErr := r.requestTimed(step.method, step.path, token, step.payload)
		if callErr != nil {
			return callErr
		}
		fmt.Printf("%s %s status=%d code=%s elapsed_ms=%d\n", step.method, step.path, status, codeOf(body), elapsed.Milliseconds())
	}
	if err := r.waitForNotifications("mhsuji", 4, 8*time.Second); err != nil {
		fmt.Println("worker_completion=not-confirmed", err)
	}
	return r.printAccountEvidence("C.1", "mhsuji", 4)
}

func (r *runner) c2down() error {
	token, err := r.login("mhsuji")
	if err != nil {
		return err
	}
	for _, id := range []int{5, 6, 7} {
		status, body, elapsed, e := r.requestTimed(http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enroll", id), token, map[string]any{})
		if e != nil {
			return e
		}
		fmt.Printf("POST /api/v1/courses/%d/enroll status=%d code=%s elapsed_ms=%d\n", id, status, codeOf(body), elapsed.Milliseconds())
	}
	time.Sleep(5 * time.Second)
	status, body, err := r.request(http.MethodGet, "/api/v1/admin/queue", r.adminToken(), nil)
	if err != nil {
		return err
	}
	var q model.QueueStats
	_ = decodeData(body, &q)
	fmt.Printf("GET /api/v1/admin/queue status=%d outbox_pending=%d queue_error_present=%t error=%s\n", status, q.OutboxPending, q.QueueError != "", q.QueueError)
	return nil
}

func (r *runner) c2up() error {
	if err := r.waitForNotifications("mhsuji", 7, 15*time.Second); err != nil {
		fmt.Println("C2 completion wait:", err)
	}
	status, body, err := r.request(http.MethodGet, "/api/v1/admin/queue", r.adminToken(), nil)
	if err != nil {
		return err
	}
	var q model.QueueStats
	_ = decodeData(body, &q)
	fmt.Printf("GET /api/v1/admin/queue status=%d outbox_pending=%d outbox_published=%d queue_error_present=%t\n", status, q.OutboxPending, q.OutboxPublished, q.QueueError != "")
	return r.printAccountEvidence("C.2", "mhsuji", 7)
}

func (r *runner) c3() error {
	token, err := r.login("mhsuji")
	if err != nil {
		return err
	}
	for _, id := range []int{1, 2, 3, 4, 5, 6} {
		status, body, elapsed, e := r.requestTimed(http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enroll", id), token, map[string]any{})
		if e != nil {
			return e
		}
		fmt.Printf("POST /api/v1/courses/%d/enroll status=%d code=%s elapsed_ms=%d\n", id, status, codeOf(body), elapsed.Milliseconds())
	}
	if err := r.waitForNotifications("mhsuji", 6, 60*time.Second); err != nil {
		fmt.Println("C3 completion wait:", err)
	}
	time.Sleep(time.Second)
	return r.printAccountEvidence("C.3", "mhsuji", 6)
}

func (r *runner) c4() error {
	token, err := r.login("salahalamat")
	if err != nil {
		return err
	}
	before, err := r.queueStats()
	if err != nil {
		return err
	}
	status, body, elapsed, err := r.requestTimed(http.MethodPost, "/api/v1/courses/2/enroll", token, map[string]any{})
	if err != nil {
		return err
	}
	fmt.Printf("POST /api/v1/courses/2/enroll status=%d code=%s elapsed_ms=%d\n", status, codeOf(body), elapsed.Milliseconds())
	deadline := time.Now().Add(20 * time.Second)
	var after model.QueueStats
	for time.Now().Before(deadline) {
		after, err = r.queueStats()
		if err == nil && queueArchived(after) > queueArchived(before) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Printf("C.4 queue before archived=%d retry=%d; after archived=%d retry=%d\n", queueArchived(before), queueRetry(before), queueArchived(after), queueRetry(after))
	return r.printAccountEvidence("C.4", "salahalamat", -1)
}

func (r *runner) c5enroll() error {
	token, err := r.login("mhsuji")
	if err != nil {
		return err
	}
	status, body, elapsed, err := r.requestTimed(http.MethodPost, "/api/v1/courses/8/enroll", token, map[string]any{})
	if err != nil {
		return err
	}
	fmt.Printf("POST /api/v1/courses/8/enroll status=%d code=%s elapsed_ms=%d\n", status, codeOf(body), elapsed.Milliseconds())
	var id int64
	var published *time.Time
	if err := r.pool.QueryRow(r.ctx, `SELECT id,published_at FROM outbox WHERE payload->>'user_id'=$1 ORDER BY id DESC LIMIT 1`, fmt.Sprint(r.userID("mhsuji"))).Scan(&id, &published); err != nil {
		return err
	}
	fmt.Printf("C.5 outbox_id=%d published_at_is_null=%t\n", id, published == nil)
	return nil
}

func (r *runner) c5verify() error { return r.printAccountEvidence("C.5", "mhsuji", -1) }

func (r *runner) c6enroll() error {
	token, err := r.login("mhsuji")
	if err != nil {
		return err
	}
	status, body, elapsed, err := r.requestTimed(http.MethodPost, "/api/v1/courses/9/enroll", token, map[string]any{})
	if err != nil {
		return err
	}
	fmt.Printf("POST /api/v1/courses/9/enroll status=%d code=%s elapsed_ms=%d\n", status, codeOf(body), elapsed.Milliseconds())
	var id int64
	if err := r.pool.QueryRow(r.ctx, `SELECT id FROM outbox WHERE payload->>'user_id'=$1 AND event_type=$2 ORDER BY id DESC LIMIT 1`, fmt.Sprint(r.userID("mhsuji")), model.EventEnrollmentCreated).Scan(&id); err != nil {
		return err
	}
	fmt.Printf("C.6 outbox_id=%d; wait for relay_done, then signal worker\n", id)
	return nil
}

func (r *runner) c6verify() error { return r.printAccountEvidence("C.6", "mhsuji", -1) }

func (r *runner) printAccountEvidence(label, username string, expected int) error {
	id := r.userID(username)
	if id == 0 {
		return errors.New("test account not found")
	}
	var outboxCount, notifications, pending int
	if err := r.pool.QueryRow(r.ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE published_at IS NULL) FROM outbox WHERE payload->>'user_id'=$1`, fmt.Sprint(id)).Scan(&outboxCount, &pending); err != nil {
		return err
	}
	if err := r.pool.QueryRow(r.ctx, `SELECT COUNT(*) FROM notifications WHERE user_id=$1`, id).Scan(&notifications); err != nil {
		return err
	}
	var types []string
	rows, err := r.pool.Query(r.ctx, `SELECT event_type FROM outbox WHERE payload->>'user_id'=$1 ORDER BY id`, fmt.Sprint(id))
	if err != nil {
		return err
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		types = append(types, v)
	}
	rows.Close()
	rows, err = r.pool.Query(r.ctx, `SELECT o.id,o.event_type,o.published_at IS NOT NULL,COALESCE(n.subject,''),n.sent_at FROM outbox o LEFT JOIN notifications n ON n.outbox_id=o.id WHERE o.payload->>'user_id'=$1 ORDER BY o.id`, fmt.Sprint(id))
	if err != nil {
		return err
	}
	for rows.Next() {
		var eventID int64
		var eventType string
		var published bool
		var subject string
		var sentAt *time.Time
		if err := rows.Scan(&eventID, &eventType, &published, &subject, &sentAt); err != nil {
			rows.Close()
			return err
		}
		fmt.Printf("%s query_row outbox_id=%d event_type=%s published=%t notification_subject=%q sent_at=%v\n", label, eventID, eventType, published, subject, sentAt)
	}
	rows.Close()
	var subjects []string
	rows, err = r.pool.Query(r.ctx, `SELECT subject FROM notifications WHERE user_id=$1 ORDER BY outbox_id`, id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		subjects = append(subjects, v)
	}
	rows.Close()
	apiCount := -1
	if token, e := r.login(username); e == nil {
		status, body, e := r.request(http.MethodGet, "/api/v1/notifications/mine", token, nil)
		if e == nil && status == http.StatusOK {
			apiCount = notificationCount(body)
		}
	}
	mailCount := r.mailLines(emailFor(username))
	fmt.Printf("%s db_outbox=%d db_notifications=%d api_notifications=%d mail_log_lines=%d pending_outbox=%d target=%d\n", label, outboxCount, notifications, apiCount, mailCount, pending, expected)
	fmt.Printf("%s outbox_types=%v\n", label, types)
	fmt.Printf("%s notification_subjects=%q\n", label, subjects)
	return nil
}

func (r *runner) waitForNotifications(username string, expected int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var got int
		if err := r.pool.QueryRow(r.ctx, `SELECT COUNT(*) FROM notifications WHERE user_id=$1`, r.userID(username)).Scan(&got); err == nil && got == expected {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("notifications did not reach %d within %s", expected, timeout)
}

func (r *runner) queueStats() (model.QueueStats, error) {
	token, err := r.adminToken(), error(nil)
	status, body, e := r.request(http.MethodGet, "/api/v1/admin/queue", token, nil)
	if e != nil {
		return model.QueueStats{}, e
	}
	if status != http.StatusOK {
		return model.QueueStats{}, fmt.Errorf("admin queue endpoint status %d: %s", status, messageOf(body))
	}
	var q model.QueueStats
	if e = decodeData(body, &q); e != nil {
		return q, e
	}
	return q, err
}

func (r *runner) adminToken() string {
	var id int
	var username string
	if err := r.pool.QueryRow(r.ctx, `SELECT id,username FROM users WHERE role='admin' ORDER BY id LIMIT 1`).Scan(&id, &username); err != nil {
		return ""
	}
	manager := helper.NewJWTManager(config.GetEnv("JWT_SECRET", ""), config.GetEnv("JWT_ISSUER", "praktikum-backend"), time.Duration(config.GetEnvInt("JWT_ACCESS_TTL_MINUTES", 15))*time.Minute)
	token, err := manager.GenerateAccess(model.User{ID: id, Username: username, Role: "admin"})
	if err != nil {
		return ""
	}
	return token
}

func (r *runner) login(username string) (string, error) {
	status, body, err := r.request(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"username": username, "password": testPassword})
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("login %s status %d: %s", username, status, messageOf(body))
	}
	var e envelope
	if err := json.Unmarshal(body, &e); err != nil {
		return "", err
	}
	var pair model.TokenPair
	if err := json.Unmarshal(e.Data, &pair); err != nil {
		return "", err
	}
	if pair.AccessToken == "" {
		return "", errors.New("login response did not contain access token")
	}
	return pair.AccessToken, nil
}

func (r *runner) request(method, path, token string, payload any) (int, []byte, error) {
	status, body, _, err := r.requestTimed(method, path, token, payload)
	return status, body, err
}
func (r *runner) requestTimed(method, path, token string, payload any) (int, []byte, time.Duration, error) {
	var reader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, 0, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(r.ctx, method, r.base+path, reader)
	if err != nil {
		return 0, nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	start := time.Now()
	resp, err := r.client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return 0, nil, elapsed, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, elapsed, err
}

func (r *runner) userID(username string) int {
	var id int
	if err := r.pool.QueryRow(r.ctx, `SELECT id FROM users WHERE username=$1`, username).Scan(&id); err != nil {
		return 0
	}
	return id
}
func (r *runner) mailLines(address string) int {
	data, err := os.ReadFile(filepath.Join("logs", "mail.log"))
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "to="+address) {
			n++
		}
	}
	return n
}
func emailFor(username string) string {
	if username == "mhsuji" {
		return "mhsuji@student.unair.ac.id"
	}
	return "salah@kampus.invalid"
}
func messageOf(data []byte) string { var e envelope; _ = json.Unmarshal(data, &e); return e.Message }
func codeOf(data []byte) string    { var e envelope; _ = json.Unmarshal(data, &e); return e.Code }
func decodeData(data []byte, target any) error {
	var e envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	return json.Unmarshal(e.Data, target)
}
func notificationCount(data []byte) int {
	var e envelope
	if json.Unmarshal(data, &e) != nil {
		return -1
	}
	var ns []model.Notification
	if json.Unmarshal(e.Data, &ns) != nil {
		return -1
	}
	return len(ns)
}
func queueArchived(q model.QueueStats) int {
	if q.Queue == nil {
		return -1
	}
	return q.Queue.Archived
}
func queueRetry(q model.QueueStats) int {
	if q.Queue == nil {
		return -1
	}
	return q.Queue.Retry
}
