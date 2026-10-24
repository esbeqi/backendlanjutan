package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const reviewTestUsers = `u.username ~ '^mhs(00[1-9]|0[1-5][0-9]|060)$'`

type reviewResponse struct {
	ID           string    `json:"id"`
	UserID       int       `json:"user_id"`
	Rating       int       `json:"rating"`
	Comment      string    `json:"comment"`
	Tags         []string  `json:"tags"`
	HelpfulCount int       `json:"helpful_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Replies      []any     `json:"replies"`
}

type reviewEnvelope struct {
	Data reviewResponse `json:"data"`
}

type reviewListEnvelope struct {
	Data []json.RawMessage `json:"data"`
}

type enrolledReviewUser struct {
	username string
	courseID int
}

func (r *runner) reviewScenario(scenario string) error {
	switch scenario {
	case "read":
		return r.reviewReadOnly()
	case "validation":
		return r.reviewValidation()
	case "write":
		return r.reviewWrite()
	case "concurrent":
		return r.reviewConcurrent()
	default:
		return fmt.Errorf("skenario review tidak dikenal %q; pilih read, validation, write, atau concurrent", scenario)
	}
}

func (r *runner) reviewReadOnly() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	username, err := r.firstReviewUser(ctx)
	if err != nil {
		return err
	}
	courseID, err := r.firstCourse(ctx)
	if err != nil {
		return err
	}
	token, _, err := r.tokenFor(ctx, username, "user")
	if err != nil {
		return err
	}
	if err := expect("daftar review read-only", r.request(http.MethodGet,
		fmt.Sprintf("/api/v1/courses/%d/reviews?limit=10", courseID), token, nil), http.StatusOK, ""); err != nil {
		return err
	}
	if err := expect("summary review read-only", r.request(http.MethodGet,
		fmt.Sprintf("/api/v1/courses/%d/reviews/summary", courseID), token, nil), http.StatusOK, ""); err != nil {
		return err
	}
	fmt.Printf("review read-only memakai course id=%d dan user %s dari PostgreSQL\n", courseID, username)
	return nil
}

func (r *runner) reviewValidation() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	username, err := r.firstReviewUser(ctx)
	if err != nil {
		return err
	}
	courseID, err := r.firstCourse(ctx)
	if err != nil {
		return err
	}
	token, _, err := r.tokenFor(ctx, username, "user")
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/api/v1/courses/%d/reviews", courseID)
	validComment := "Review validation fixture"
	cases := []struct {
		label   string
		payload any
	}{
		{"rating di luar batas", map[string]any{"rating": 6, "comment": validComment}},
		{"comment terlalu pendek", map[string]any{"rating": 5, "comment": "singkat"}},
		{"tag lebih dari lima", map[string]any{"rating": 5, "comment": validComment, "tags": []string{"a1", "b2", "c3", "d4", "e5", "f6"}}},
	}
	for _, test := range cases {
		if err := expect(test.label, r.request(http.MethodPost, path, token, test.payload), fiber.StatusUnprocessableEntity, ""); err != nil {
			return err
		}
	}
	if err := expect("tolak payload NoSQL", r.request(http.MethodPost, path, token,
		map[string]any{"rating": 5, "comment": map[string]string{"$ne": ""}}), fiber.StatusBadRequest, ""); err != nil {
		return err
	}
	if err := expect("tolak id review tidak valid", r.request(http.MethodGet,
		"/api/v1/reviews/not-an-object-id", token, nil), fiber.StatusBadRequest, ""); err != nil {
		return err
	}
	queryPath := fmt.Sprintf("/api/v1/courses/%d/reviews?tag=%s", courseID,
		url.QueryEscape(`{"$ne":""}`))
	queryResult := r.request(http.MethodGet, queryPath, token, nil)
	if err := expect("tag query diperlakukan sebagai string", queryResult, http.StatusOK, ""); err != nil {
		return err
	}
	var queryEnvelope reviewListEnvelope
	if err := json.Unmarshal(queryResult.body, &queryEnvelope); err != nil {
		return fmt.Errorf("decode hasil query tag uji: %w", err)
	}
	if len(queryEnvelope.Data) != 0 {
		return fmt.Errorf("tag query NoSQL literal menghasilkan %d review; ingin 0", len(queryEnvelope.Data))
	}
	missingCourseID, err := r.missingCourseID(ctx)
	if err != nil {
		return err
	}
	validPayload := map[string]any{"rating": 5, "comment": "Review validation fixture"}
	if err := expect("course yang tidak ada saat create", r.request(http.MethodPost,
		fmt.Sprintf("/api/v1/courses/%d/reviews", missingCourseID), token, validPayload), fiber.StatusNotFound, ""); err != nil {
		return err
	}
	if err := expect("course yang tidak ada", r.request(http.MethodGet,
		fmt.Sprintf("/api/v1/courses/%d/reviews/summary", missingCourseID), token, nil), fiber.StatusNotFound, ""); err != nil {
		return err
	}
	nonEnrolledUser, nonEnrolledCourse, err := r.nonEnrolledReviewUser(ctx)
	if err != nil {
		return err
	}
	nonEnrolledToken, _, err := r.tokenFor(ctx, nonEnrolledUser, "user")
	if err != nil {
		return err
	}
	if err := expect("create oleh user non-enrolled", r.request(http.MethodPost,
		fmt.Sprintf("/api/v1/courses/%d/reviews", nonEnrolledCourse), nonEnrolledToken, validPayload), fiber.StatusForbidden, "NOT_ENROLLED"); err != nil {
		return err
	}
	fmt.Printf("review validation memakai course id=%d dan user uji dari PostgreSQL; seluruh kasus menolak request atau membaca data\n", courseID)
	return nil
}

func (r *runner) reviewWrite() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	courseID, ownerToken, ownerID, created, err := r.createTemporaryReview(ctx)
	if err != nil {
		return err
	}
	reviewID := created.ID
	deleted := false
	defer func() {
		if deleted {
			return
		}
		cleanup := r.request(http.MethodDelete, "/api/v1/reviews/"+reviewID, ownerToken, nil)
		if cleanup.err != nil || cleanup.status != fiber.StatusNoContent {
			fmt.Printf("CLEANUP NEEDED: review uji %s belum terhapus melalui API (status=%d, error=%v)\n", reviewID, cleanup.status, cleanup.err)
		}
	}()

	if created.Tags == nil || strings.Join(created.Tags, ",") != "practical,api" {
		return fmt.Errorf("tag hasil normalisasi = %v, ingin [practical api]", created.Tags)
	}
	initialCreatedAt := created.CreatedAt
	if initialCreatedAt.IsZero() {
		return fmt.Errorf("response create review tidak memuat created_at")
	}
	if err := expect("duplicate review", r.request(http.MethodPost,
		fmt.Sprintf("/api/v1/courses/%d/reviews", courseID), ownerToken,
		map[string]any{"rating": 5, "comment": "Review duplicate fixture", "tags": []string{"duplicate"}}), fiber.StatusConflict, "ALREADY_REVIEWED"); err != nil {
		return err
	}
	if err := expect("ambil review", r.request(http.MethodGet, "/api/v1/reviews/"+reviewID, ownerToken, nil), http.StatusOK, ""); err != nil {
		return err
	}
	if err := expect("daftar review setelah create", r.request(http.MethodGet,
		fmt.Sprintf("/api/v1/courses/%d/reviews", courseID), ownerToken, nil), http.StatusOK, ""); err != nil {
		return err
	}
	if err := expect("summary setelah create", r.request(http.MethodGet,
		fmt.Sprintf("/api/v1/courses/%d/reviews/summary", courseID), ownerToken, nil), http.StatusOK, ""); err != nil {
		return err
	}

	otherUsername, err := r.otherReviewUser(ctx, ownerID)
	if err != nil {
		return err
	}
	otherToken, otherID, err := r.tokenFor(ctx, otherUsername, "user")
	if err != nil {
		return err
	}
	if err := expect("update bukan pemilik", r.request(http.MethodPatch, "/api/v1/reviews/"+reviewID,
		otherToken, map[string]any{"comment": "Perubahan yang bukan milik user"}), fiber.StatusForbidden, ""); err != nil {
		return err
	}
	time.Sleep(2 * time.Millisecond)
	updatedResult := r.request(http.MethodPatch, "/api/v1/reviews/"+reviewID, ownerToken,
		map[string]any{"comment": "Review diperbarui untuk tes endpoint"})
	if err := expect("update oleh pemilik", updatedResult, http.StatusOK, ""); err != nil {
		return err
	}
	var updated reviewEnvelope
	if err := json.Unmarshal(updatedResult.body, &updated); err != nil {
		return fmt.Errorf("membaca response update review: %w", err)
	}
	if updated.Data.CreatedAt.IsZero() || !updated.Data.CreatedAt.Equal(initialCreatedAt) || !updated.Data.UpdatedAt.After(updated.Data.CreatedAt) {
		return fmt.Errorf("timestamp review tidak sesuai: created_at=%s updated_at=%s", updated.Data.CreatedAt, updated.Data.UpdatedAt)
	}
	if err := expect("reply review", r.request(http.MethodPost, "/api/v1/reviews/"+reviewID+"/replies",
		otherToken, map[string]string{"body": "Balasan uji review"}), fiber.StatusCreated, ""); err != nil {
		return err
	}
	if err := expect("vote pada review sendiri", r.request(http.MethodPost,
		"/api/v1/reviews/"+reviewID+"/helpful", ownerToken, map[string]any{}), fiber.StatusConflict, "OWN_REVIEW"); err != nil {
		return err
	}
	if err := expect("helpful vote pertama", r.request(http.MethodPost,
		"/api/v1/reviews/"+reviewID+"/helpful", otherToken, map[string]any{}), http.StatusOK, ""); err != nil {
		return err
	}
	if err := expect("helpful vote duplikat", r.request(http.MethodPost,
		"/api/v1/reviews/"+reviewID+"/helpful", otherToken, map[string]any{}), fiber.StatusConflict, "ALREADY_VOTED"); err != nil {
		return err
	}
	if err := expect("hapus oleh bukan pemilik", r.request(http.MethodDelete,
		"/api/v1/reviews/"+reviewID, otherToken, nil), fiber.StatusForbidden, ""); err != nil {
		return err
	}
	adminToken, _, err := r.tokenFor(ctx, otherUsername, "admin")
	if err != nil {
		return err
	}
	if err := expect("hapus oleh moderator", r.request(http.MethodDelete,
		"/api/v1/reviews/"+reviewID, adminToken, nil), fiber.StatusNoContent, ""); err != nil {
		return err
	}
	deleted = true
	if err := expect("review terhapus tidak ditemukan", r.request(http.MethodGet,
		"/api/v1/reviews/"+reviewID, ownerToken, nil), fiber.StatusNotFound, ""); err != nil {
		return err
	}
	fmt.Printf("review write memakai course id=%d, owner id=%d dan voter id=%d dari database; dokumen uji berhasil dihapus melalui endpoint\n", courseID, ownerID, otherID)
	return nil
}

func (r *runner) reviewConcurrent() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	courseID, ownerToken, ownerID, created, err := r.createTemporaryReview(ctx)
	if err != nil {
		return err
	}
	reviewID := created.ID
	deleted := false
	defer func() {
		if deleted {
			return
		}
		cleanup := r.request(http.MethodDelete, "/api/v1/reviews/"+reviewID, ownerToken, nil)
		if cleanup.err != nil || cleanup.status != fiber.StatusNoContent {
			fmt.Printf("CLEANUP NEEDED: review uji %s belum terhapus melalui API (status=%d, error=%v)\n", reviewID, cleanup.status, cleanup.err)
		}
	}()

	users, err := r.reviewUsernames(ctx)
	if err != nil {
		return err
	}
	voters := make([]struct {
		username string
		token    string
	}, 0, len(users))
	for _, username := range users {
		token, id, err := r.tokenFor(ctx, username, "user")
		if err != nil {
			return err
		}
		if id == ownerID {
			continue
		}
		voters = append(voters, struct {
			username string
			token    string
		}{username: username, token: token})
	}
	if len(voters) < 31 {
		return fmt.Errorf("prasyarat review concurrent: perlu 31 user uji selain pemilik; ditemukan %d", len(voters))
	}

	const repeatedRequests = 20
	jobs := make([]func() result, repeatedRequests)
	for i := range jobs {
		token := voters[0].token
		jobs[i] = func() result {
			return r.request(http.MethodPost, "/api/v1/reviews/"+reviewID+"/helpful", token, map[string]any{})
		}
	}
	results, elapsed := r.fire(jobs)
	counts := tally(results)
	if counts["200"] != 1 || counts["409 ALREADY_VOTED"] != repeatedRequests-1 || len(counts) != 2 {
		return fmt.Errorf("helpful vote serentak user sama: hasil %v, ingin 1 sukses dan %d duplicate", counts, repeatedRequests-1)
	}
	fmt.Printf("PASS %-34s sukses=1 duplicate=%d waktu=%s\n", "helpful vote concurrent once", repeatedRequests-1, elapsed)
	if err := r.assertHelpfulCount(reviewID, voters[0].token, 1); err != nil {
		return err
	}

	jobs = make([]func() result, 30)
	for i := range jobs {
		token := voters[i+1].token
		jobs[i] = func() result {
			return r.request(http.MethodPost, "/api/v1/reviews/"+reviewID+"/helpful", token, map[string]any{})
		}
	}
	results, elapsed = r.fire(jobs)
	counts = tally(results)
	if counts["200"] != 30 || len(counts) != 1 {
		return fmt.Errorf("helpful vote serentak user berbeda: hasil %v, ingin 30 sukses", counts)
	}
	fmt.Printf("PASS %-34s sukses=30 waktu=%s\n", "helpful votes concurrent distinct", elapsed)
	if err := r.assertHelpfulCount(reviewID, voters[1].token, 31); err != nil {
		return err
	}

	cleanup := r.request(http.MethodDelete, "/api/v1/reviews/"+reviewID, ownerToken, nil)
	if err := expect("hapus review concurrent uji", cleanup, fiber.StatusNoContent, ""); err != nil {
		return err
	}
	deleted = true
	fmt.Printf("review concurrent memakai course id=%d dan %d user uji dari database; dokumen uji dihapus melalui endpoint\n", courseID, len(voters))
	return nil
}

func (r *runner) createTemporaryReview(ctx context.Context) (courseID int, ownerToken string, ownerID int, created reviewResponse, err error) {
	pairs, err := r.enrolledReviewUsers(ctx)
	if err != nil {
		return 0, "", 0, reviewResponse{}, err
	}
	for _, pair := range pairs {
		token, id, tokenErr := r.tokenFor(ctx, pair.username, "user")
		if tokenErr != nil {
			return 0, "", 0, reviewResponse{}, tokenErr
		}
		resp := r.request(http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/reviews", pair.courseID), token,
			map[string]any{"rating": 5, "comment": "Review uji otomatis modul sepuluh", "tags": []string{" PRACTICAL ", "API", "practical"}})
		if resp.err != nil {
			return 0, "", 0, reviewResponse{}, fmt.Errorf("buat review uji: %w", resp.err)
		}
		if resp.status == fiber.StatusConflict && resp.code == "ALREADY_REVIEWED" {
			continue
		}
		if resp.status != fiber.StatusCreated {
			return 0, "", 0, reviewResponse{}, fmt.Errorf("buat review uji untuk course id=%d dan user %s: menerima %d %q, mengharapkan 201", pair.courseID, pair.username, resp.status, resp.code)
		}
		var envelope reviewEnvelope
		if err := json.Unmarshal(resp.body, &envelope); err != nil {
			return 0, "", 0, reviewResponse{}, fmt.Errorf("decode hasil create review: %w", err)
		}
		if envelope.Data.ID == "" {
			return 0, "", 0, reviewResponse{}, fmt.Errorf("response create review tidak memuat data.id")
		}
		fmt.Printf("PASS %-34s 201 review id=%s course id=%d user=%s\n", "buat review sementara", envelope.Data.ID, pair.courseID, pair.username)
		return pair.courseID, token, id, envelope.Data, nil
	}
	return 0, "", 0, reviewResponse{}, fmt.Errorf("prasyarat review: tidak ditemukan pasangan user-course enrolled tanpa review; database hanya dibaca sebelum percobaan create")
}

func (r *runner) enrolledReviewUsers(ctx context.Context) ([]enrolledReviewUser, error) {
	rows, err := r.pool.Query(ctx, `SELECT u.username,e.course_id FROM enrollments e JOIN users u ON u.id=e.user_id WHERE `+reviewTestUsers+` ORDER BY u.username,e.course_id`)
	if err != nil {
		return nil, fmt.Errorf("mencari user-course enrolled: %w", err)
	}
	defer rows.Close()
	pairs := make([]enrolledReviewUser, 0)
	for rows.Next() {
		var pair enrolledReviewUser
		if err := rows.Scan(&pair.username, &pair.courseID); err != nil {
			return nil, fmt.Errorf("membaca pasangan enrollment: %w", err)
		}
		pairs = append(pairs, pair)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("membaca pasangan enrollment: %w", err)
	}
	return pairs, nil
}

func (r *runner) reviewUsernames(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT username FROM users u WHERE `+reviewTestUsers+` ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("mencari akun review uji: %w", err)
	}
	defer rows.Close()
	users := make([]string, 0, 60)
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, fmt.Errorf("membaca akun review uji: %w", err)
		}
		users = append(users, username)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("membaca akun review uji: %w", err)
	}
	return users, nil
}

func (r *runner) firstReviewUser(ctx context.Context) (string, error) {
	var username string
	err := r.pool.QueryRow(ctx, `SELECT username FROM users u WHERE `+reviewTestUsers+` ORDER BY username LIMIT 1`).Scan(&username)
	if err != nil {
		return "", fmt.Errorf("prasyarat review: tidak menemukan akun uji mahasiswa: %w", err)
	}
	return username, nil
}

func (r *runner) otherReviewUser(ctx context.Context, excludedID int) (string, error) {
	var username string
	err := r.pool.QueryRow(ctx, `SELECT username FROM users u WHERE `+reviewTestUsers+` AND id<>$1 ORDER BY username LIMIT 1`, excludedID).Scan(&username)
	if err != nil {
		return "", fmt.Errorf("prasyarat review: tidak menemukan user kedua: %w", err)
	}
	return username, nil
}

func (r *runner) nonEnrolledReviewUser(ctx context.Context) (string, int, error) {
	var username string
	var courseID int
	err := r.pool.QueryRow(ctx, `SELECT u.username,c.id FROM users u CROSS JOIN courses c
		LEFT JOIN enrollments e ON e.user_id=u.id AND e.course_id=c.id
		WHERE `+reviewTestUsers+` AND e.id IS NULL ORDER BY u.username,c.id LIMIT 1`).Scan(&username, &courseID)
	if err != nil {
		return "", 0, fmt.Errorf("prasyarat review validation: tidak ditemukan user uji non-enrolled/course: %w", err)
	}
	return username, courseID, nil
}

func (r *runner) firstCourse(ctx context.Context) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, "SELECT id FROM courses ORDER BY id LIMIT 1").Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("prasyarat review: tidak menemukan course di PostgreSQL: %w", err)
	}
	return id, nil
}

func (r *runner) missingCourseID(ctx context.Context) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, "SELECT COALESCE(MAX(id),0)+1 FROM courses").Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("mencari ID course yang tidak digunakan: %w", err)
	}
	return id, nil
}

func (r *runner) assertHelpfulCount(reviewID, token string, want int) error {
	var envelope reviewEnvelope
	got := r.request(http.MethodGet, "/api/v1/reviews/"+reviewID, token, nil)
	if got.err != nil {
		return fmt.Errorf("membaca counter helpful: %w", got.err)
	}
	if got.status != http.StatusOK {
		return fmt.Errorf("membaca review untuk memeriksa counter helpful: menerima %d, ingin 200", got.status)
	}
	if err := json.Unmarshal(got.body, &envelope); err != nil {
		return fmt.Errorf("decode counter helpful: %w", err)
	}
	if envelope.Data.HelpfulCount != want {
		return fmt.Errorf("helpful_count=%d, ingin %d", envelope.Data.HelpfulCount, want)
	}
	return nil
}
