// Command loadtest menyiapkan dan menjalankan skenario penerimaan Modul 9.
// Jalankan hanya terhadap database praktikum yang memang disiapkan untuk uji.
// Command ini tidak menghapus atau mereset data. Reset dan persiapan keadaan
// awal tetap dilakukan operator sesuai reset.sql dan prasyarat di -h.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"latihan-fiber/app/model"
	"latihan-fiber/config"
	"latihan-fiber/database"
	"latihan-fiber/helper"
)

type runner struct {
	baseURL string
	client  *http.Client
	pool    *pgxpool.Pool
	jwt     *helper.JWTManager
	tokens  map[int]string
}

type result struct {
	status int
	code   string
	body   []byte
	err    error
}

func main() {
	mode := flag.String("mode", "", "crowd | credits | setup-swap | swap | functional | health | logs")
	baseURL := flag.String("url", "", "base URL, default http://localhost:$APP_PORT")
	logPath := flag.String("log", filepath.Join("logs", "app.log"), "app log path for -mode logs")
	flag.Parse()

	if *mode == "health" {
		checkHealth(defaultURL(*baseURL))
		return
	}
	if *mode == "logs" {
		checkLogs(*logPath)
		return
	}
	if *mode != "crowd" && *mode != "credits" && *mode != "setup-swap" && *mode != "swap" && *mode != "functional" {
		fatalf("mode tidak dikenal; gunakan -h untuk bantuan")
	}

	config.LoadEnv()
	pool, err := database.NewPool(context.Background())
	if err != nil {
		fatalf("koneksi database praktikum: %v", err)
	}
	defer pool.Close()
	secret := config.GetEnv("JWT_SECRET", "")
	if len(secret) < 32 {
		fatalf("JWT_SECRET harus sama dengan server dan minimal 32 karakter")
	}
	r := &runner{
		baseURL: defaultURL(*baseURL),
		client:  &http.Client{Timeout: 15 * time.Second},
		pool:    pool,
		jwt:     helper.NewJWTManager(secret, config.GetEnv("JWT_ISSUER", "praktikum-backend"), 15*time.Minute),
		tokens:  make(map[int]string),
	}

	var runErr error
	switch *mode {
	case "crowd":
		runErr = r.crowd()
	case "credits":
		runErr = r.credits()
	case "setup-swap":
		runErr = r.setupSwap()
	case "swap":
		runErr = r.swap()
	case "functional":
		runErr = r.functional()
	}
	if runErr != nil {
		fatalf("%v", runErr)
	}
}

func defaultURL(value string) string {
	if value != "" {
		return strings.TrimRight(value, "/")
	}
	return "http://localhost:" + config.GetEnv("APP_PORT", "3000")
}

func (r *runner) tokenFor(ctx context.Context, username, role string) (string, int, error) {
	var id int
	var actualUsername string
	err := r.pool.QueryRow(ctx, "SELECT id, username FROM users WHERE username=$1", username).Scan(&id, &actualUsername)
	if err != nil {
		return "", 0, fmt.Errorf("lookup user %s: %w", username, err)
	}
	if token, ok := r.tokens[id]; ok && role == "user" {
		return token, id, nil
	}
	token, err := r.jwt.GenerateAccess(model.User{ID: id, Username: actualUsername, Role: role})
	if err != nil {
		return "", 0, err
	}
	if role == "user" {
		r.tokens[id] = token
	}
	return token, id, nil
}

func (r *runner) userIDs(ctx context.Context, first, last int) ([]int, error) {
	ids := make([]int, 0, last-first+1)
	for i := first; i <= last; i++ {
		_, id, err := r.tokenFor(ctx, fmt.Sprintf("mhs%03d", i), "user")
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *runner) request(method, path, token string, payload any) result {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return result{err: err}
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, r.baseURL+path, body)
	if err != nil {
		return result{err: err}
	}
	if payload != nil {
		req.Header.Set("Content-Type", fiber.MIMEApplicationJSON)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return result{err: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return result{status: resp.StatusCode, err: err}
	}
	out := result{status: resp.StatusCode}
	out.body = data
	if len(data) > 0 {
		var envelope struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(data, &envelope)
		out.code = envelope.Code
	}
	return out
}

func (r *runner) fire(jobs []func() result) ([]result, time.Duration) {
	start := make(chan struct{})
	results := make([]result, len(jobs))
	var wg sync.WaitGroup
	for i := range jobs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = jobs[i]()
		}(i)
	}
	started := time.Now()
	close(start)
	wg.Wait()
	return results, time.Since(started)
}

func tally(results []result) map[string]int {
	counts := make(map[string]int)
	for _, item := range results {
		key := fmt.Sprintf("%d", item.status)
		if item.status == fiber.StatusConflict && item.code != "" {
			key += " " + item.code
		}
		if item.err != nil {
			key = "transport error"
		}
		counts[key]++
	}
	return counts
}

func (r *runner) course(ctx context.Context, code string) (id, credits, quota int, err error) {
	err = r.pool.QueryRow(ctx, "SELECT id, credits, quota FROM courses WHERE code=$1", code).Scan(&id, &credits, &quota)
	return
}

func (r *runner) assertEmpty(ctx context.Context, userIDs []int, courseCodes ...string) error {
	for _, userID := range userIDs {
		var count int
		if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM enrollments WHERE user_id=$1", userID).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("prasyarat: user id %d masih memiliki %d enrollment; reset data uji sebelum melanjutkan", userID, count)
		}
	}
	for _, code := range courseCodes {
		id, _, _, err := r.course(ctx, code)
		if err != nil {
			return err
		}
		count, actual, err := r.courseCounts(ctx, id)
		if err != nil {
			return err
		}
		if count != 0 || actual != 0 {
			return fmt.Errorf("prasyarat: %s sudah memiliki enrolled_count=%d dan %d baris; reset data uji", code, count, actual)
		}
	}
	return nil
}

func (r *runner) courseCounts(ctx context.Context, courseID int) (enrolledCount, rows int, err error) {
	err = r.pool.QueryRow(ctx, `SELECT c.enrolled_count, COUNT(e.id)
		FROM courses c LEFT JOIN enrollments e ON e.course_id=c.id WHERE c.id=$1 GROUP BY c.id`, courseID).
		Scan(&enrolledCount, &rows)
	return
}

func expect(label string, got result, status int, code string) error {
	if got.err != nil {
		return fmt.Errorf("%s: request gagal: %w", label, got.err)
	}
	if got.status != status || (code != "" && got.code != code) {
		return fmt.Errorf("%s: menerima %d %q, mengharapkan %d %q", label, got.status, got.code, status, code)
	}
	fmt.Printf("PASS %-34s %d %s\n", label, got.status, got.code)
	return nil
}

func (r *runner) crowd() error {
	ctx := context.Background()
	ids, err := r.userIDs(ctx, 1, 60)
	if err != nil {
		return err
	}
	courseID, _, quota, err := r.course(ctx, "IF399")
	if err != nil {
		return err
	}
	if quota != 5 {
		return fmt.Errorf("IF399 quota=%d, target C.1 mensyaratkan 5", quota)
	}
	before, rows, err := r.courseCounts(ctx, courseID)
	if err != nil {
		return err
	}
	if before != rows || before > quota {
		return fmt.Errorf("prasyarat: IF399 count tidak konsisten (%d versus %d)", before, rows)
	}
	if before != 0 && before != quota {
		return fmt.Errorf("prasyarat: IF399 memiliki %d peserta; reset untuk C.1 atau isi penuh untuk C.5", before)
	}
	jobs := make([]func() result, len(ids))
	for i := range ids {
		name := fmt.Sprintf("mhs%03d", i+1)
		token, _, err := r.tokenFor(ctx, name, "user")
		if err != nil {
			return err
		}
		jobs[i] = func() result {
			return r.request(http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enroll", courseID), token, map[string]any{})
		}
	}
	results, elapsed := r.fire(jobs)
	counts := tally(results)
	fmt.Printf("C.1 crowd: %v; waktu=%s\n", counts, elapsed)
	wantCreated, wantFull, wantKinds := 5, 55, 2
	if before == quota {
		wantCreated, wantFull = 0, 60
		wantKinds = 1
	}
	if counts["201"] != wantCreated || counts["409 COURSE_FULL"] != wantFull || len(counts) != wantKinds {
		return fmt.Errorf("hasil crowd %d sukses dan %d penuh tidak sesuai target %d sukses dan %d COURSE_FULL", counts["201"], counts["409 COURSE_FULL"], wantCreated, wantFull)
	}
	actualCount, rows, err := r.courseCounts(ctx, courseID)
	if err != nil {
		return err
	}
	fmt.Printf("IF399 enrolled_count=%d, enrollment rows=%d\n", actualCount, rows)
	if actualCount != 5 || rows != 5 {
		return fmt.Errorf("IF399 tidak konsisten")
	}
	return nil
}

func (r *runner) credits() error {
	ctx := context.Background()
	ids, err := r.userIDs(ctx, 1, 10)
	if err != nil {
		return err
	}
	codes := []string{"IF101", "IF102", "IF301", "IF302", "IF303", "IF304", "IF305", "IF306", "IF307", "IF308", "IF309", "IF310"}
	if err := r.assertEmpty(ctx, ids, codes...); err != nil {
		return err
	}
	type target struct {
		id    int
		token string
	}
	targets := make([]target, 0, 120)
	for i := 0; i < 10; i++ {
		token, _, err := r.tokenFor(ctx, fmt.Sprintf("mhs%03d", i+1), "user")
		if err != nil {
			return err
		}
		for _, code := range codes {
			courseID, credits, _, err := r.course(ctx, code)
			if err != nil {
				return err
			}
			if credits != 3 {
				return fmt.Errorf("prasyarat: %s memiliki %d SKS, skenario C.2 mensyaratkan 3", code, credits)
			}
			targets = append(targets, target{id: courseID, token: token})
		}
	}
	jobs := make([]func() result, len(targets))
	for i, t := range targets {
		t := t
		jobs[i] = func() result {
			return r.request(http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enroll", t.id), t.token, map[string]any{})
		}
	}
	results, elapsed := r.fire(jobs)
	counts := tally(results)
	fmt.Printf("C.2 credits: %v; waktu=%s\n", counts, elapsed)
	if counts["201"] != 80 || counts["409 CREDIT_LIMIT_EXCEEDED"] != 40 || len(counts) != 2 {
		return fmt.Errorf("hasil credits tidak sesuai target 80x201 dan 40x409 CREDIT_LIMIT_EXCEEDED")
	}
	for i, userID := range ids {
		var total int
		if err := r.pool.QueryRow(ctx, `SELECT COALESCE(SUM(c.credits),0) FROM enrollments e
			JOIN courses c ON c.id=e.course_id WHERE e.user_id=$1`, userID).Scan(&total); err != nil {
			return err
		}
		if total > 24 {
			return fmt.Errorf("mhs%03d memiliki %d SKS, melebihi 24", i+1, total)
		}
	}
	fmt.Println("seluruh 10 pengguna berada pada atau di bawah 24 SKS")
	return nil
}

func (r *runner) setupSwap() error {
	ctx := context.Background()
	ids, err := r.userIDs(ctx, 1, 40)
	if err != nil {
		return err
	}
	if err := r.assertEmpty(ctx, ids, "IF101", "IF102"); err != nil {
		return err
	}
	fromID, _, _, err := r.course(ctx, "IF101")
	if err != nil {
		return err
	}
	toID, _, _, err := r.course(ctx, "IF102")
	if err != nil {
		return err
	}
	for i := 1; i <= 40; i++ {
		code := fromID
		if i > 20 {
			code = toID
		}
		token, _, err := r.tokenFor(ctx, fmt.Sprintf("mhs%03d", i), "user")
		if err != nil {
			return err
		}
		resp := r.request(http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enroll", code), token, map[string]any{})
		if err := expect(fmt.Sprintf("setup mhs%03d", i), resp, fiber.StatusCreated, ""); err != nil {
			return err
		}
	}
	fmt.Println("swap precondition ready: mhs001-020 in IF101; mhs021-040 in IF102")
	return nil
}

func (r *runner) swap() error {
	ctx := context.Background()
	fromID, _, _, err := r.course(ctx, "IF101")
	if err != nil {
		return err
	}
	toID, _, _, err := r.course(ctx, "IF102")
	if err != nil {
		return err
	}
	for i := 1; i <= 40; i++ {
		userID := 0
		_, userID, err = r.tokenFor(ctx, fmt.Sprintf("mhs%03d", i), "user")
		if err != nil {
			return err
		}
		from := fromID
		if i > 20 {
			from = toID
		}
		if !r.hasEnrollment(ctx, userID, from) {
			return fmt.Errorf("prasyarat C.3: mhs%03d harus terdaftar di course sumber sebelum -mode swap; jalankan reset dan -mode setup-swap", i)
		}
	}
	jobs := make([]func() result, 40)
	for i := 1; i <= 40; i++ {
		from, to := fromID, toID
		if i > 20 {
			from, to = toID, fromID
		}
		token, _, err := r.tokenFor(ctx, fmt.Sprintf("mhs%03d", i), "user")
		if err != nil {
			return err
		}
		jobs[i-1] = func() result {
			return r.request(http.MethodPost, "/api/v1/enrollments/swap", token,
				map[string]int{"from_course_id": from, "to_course_id": to})
		}
	}
	var deadlocksBefore, deadlocksAfter int64
	if err := r.pool.QueryRow(ctx, "SELECT deadlocks FROM pg_stat_database WHERE datname=current_database()").Scan(&deadlocksBefore); err != nil {
		return fmt.Errorf("read PostgreSQL deadlock counter before swap: %w", err)
	}
	results, elapsed := r.fire(jobs)
	if err := r.pool.QueryRow(ctx, "SELECT deadlocks FROM pg_stat_database WHERE datname=current_database()").Scan(&deadlocksAfter); err != nil {
		return fmt.Errorf("read PostgreSQL deadlock counter after swap: %w", err)
	}
	counts := tally(results)
	fmt.Printf("C.3 swap: %v; waktu=%s; deadlocks_delta=%d\n", counts, elapsed, deadlocksAfter-deadlocksBefore)
	if counts["200"] != 40 || len(counts) != 1 || elapsed >= time.Second || deadlocksAfter > deadlocksBefore {
		return fmt.Errorf("hasil swap tidak memenuhi target 40x200, tanpa deadlock, dan di bawah 1 detik")
	}
	for _, id := range []int{fromID, toID} {
		count, rows, err := r.courseCounts(ctx, id)
		if err != nil {
			return err
		}
		fmt.Printf("course id=%d enrolled_count=%d enrollment rows=%d\n", id, count, rows)
		if count != rows || count != 20 {
			return fmt.Errorf("course id=%d harus konsisten pada 20 peserta (aktual enrolled_count=%d, rows=%d)", id, count, rows)
		}
	}
	return nil
}

func (r *runner) functional() error {
	ctx := context.Background()
	ids, err := r.userIDs(ctx, 1, 7)
	if err != nil {
		return err
	}
	if err := r.assertEmpty(ctx, ids, "IF101", "IF102", "IF399"); err != nil {
		return err
	}
	if399, _, quota, err := r.course(ctx, "IF399")
	if err != nil {
		return err
	}
	if quota != 5 {
		return fmt.Errorf("prasyarat: IF399 quota=%d, target skenario perlu 5", quota)
	}
	if101, _, _, err := r.course(ctx, "IF101")
	if err != nil {
		return err
	}
	if102, _, _, err := r.course(ctx, "IF102")
	if err != nil {
		return err
	}
	token1, user1, err := r.tokenFor(ctx, "mhs001", "user")
	if err != nil {
		return err
	}
	token2, _, err := r.tokenFor(ctx, "mhs002", "user")
	if err != nil {
		return err
	}
	token3, _, err := r.tokenFor(ctx, "mhs003", "user")
	if err != nil {
		return err
	}
	token4, _, err := r.tokenFor(ctx, "mhs004", "user")
	if err != nil {
		return err
	}
	token5, _, err := r.tokenFor(ctx, "mhs005", "user")
	if err != nil {
		return err
	}
	token6, _, err := r.tokenFor(ctx, "mhs006", "user")
	if err != nil {
		return err
	}
	token7, _, err := r.tokenFor(ctx, "mhs007", "user")
	if err != nil {
		return err
	}
	if err := expect("enrollment berhasil", r.request(http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enroll", if101), token1, map[string]any{}), 201, ""); err != nil {
		return err
	}
	if err := expect("enrollment duplikat", r.request(http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enroll", if101), token1, map[string]any{}), 409, "ALREADY_ENROLLED"); err != nil {
		return err
	}
	for i, token := range []string{token2, token3, token4, token5, token6} {
		if err := expect(fmt.Sprintf("isi kursi IF399 %d", i+1), r.request(http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enroll", if399), token, map[string]any{}), 201, ""); err != nil {
			return err
		}
	}
	if err := expect("kelas penuh", r.request(http.MethodPost, fmt.Sprintf("/api/v1/courses/%d/enroll", if399), token1, map[string]any{}), 409, "COURSE_FULL"); err != nil {
		return err
	}
	if err := expect("drop bukan milik user", r.request(http.MethodDelete, fmt.Sprintf("/api/v1/courses/%d/enroll", if399), token7, nil), 409, "NOT_ENROLLED"); err != nil {
		return err
	}
	if err := expect("swap ke kelas penuh", r.request(http.MethodPost, "/api/v1/enrollments/swap", token1, map[string]int{"from_course_id": if101, "to_course_id": if399}), 409, "COURSE_FULL"); err != nil {
		return err
	}
	if !r.hasEnrollment(ctx, user1, if101) {
		return fmt.Errorf("atomicity swap gagal: mhs001 tidak lagi di IF101 setelah swap ditolak")
	}
	if err := expect("swap berhasil", r.request(http.MethodPost, "/api/v1/enrollments/swap", token1, map[string]int{"from_course_id": if101, "to_course_id": if102}), 200, ""); err != nil {
		return err
	}
	if r.hasEnrollment(ctx, user1, if101) || !r.hasEnrollment(ctx, user1, if102) {
		return fmt.Errorf("swap sukses tidak memindahkan enrollment mhs001 dari IF101 ke IF102")
	}
	var mismatches []string
	if err := expect("swap course sama (C.4)", r.request(http.MethodPost, "/api/v1/enrollments/swap", token1, map[string]int{"from_course_id": if102, "to_course_id": if102}), 422, "VALIDATION_ERROR"); err != nil {
		fmt.Println("KNOWN SPEC MISMATCH:", err)
		mismatches = append(mismatches, err.Error())
	}
	adminToken, _, err := r.tokenFor(ctx, "mhs007", "admin")
	if err != nil {
		return err
	}
	_, _, _, version, err := r.courseVersion(ctx, if399)
	if err != nil {
		return err
	}
	updated := r.request(http.MethodPatch, fmt.Sprintf("/api/v1/courses/%d", if399), adminToken, map[string]int{"quota": 6, "version": version})
	if err := expect("update kuota version baru", updated, 200, ""); err != nil {
		return err
	}
	newVersion, err := responseVersion(updated)
	if err != nil {
		return err
	}
	if newVersion != version+1 {
		return fmt.Errorf("version response=%d, diharapkan %d", newVersion, version+1)
	}
	if err := expect("update kuota version stale", r.request(http.MethodPatch, fmt.Sprintf("/api/v1/courses/%d", if399), adminToken, map[string]int{"quota": 7, "version": version}), 409, "VERSION_CONFLICT"); err != nil {
		return err
	}
	if err := expect("kuota di bawah peserta", r.request(http.MethodPatch, fmt.Sprintf("/api/v1/courses/%d", if399), adminToken, map[string]int{"quota": 4, "version": newVersion}), 409, "QUOTA_BELOW_ENROLLED"); err != nil {
		return err
	}
	if err := expect("user biasa ubah kuota", r.request(http.MethodPatch, fmt.Sprintf("/api/v1/courses/%d", if399), token6, map[string]int{"quota": 8, "version": newVersion}), 403, ""); err != nil {
		return err
	}
	for _, courseID := range []int{if101, if102, if399} {
		count, rows, err := r.courseCounts(ctx, courseID)
		if err != nil {
			return err
		}
		if count != rows {
			return fmt.Errorf("count tidak konsisten course id=%d (%d != %d)", courseID, count, rows)
		}
	}
	if len(mismatches) != 0 {
		return fmt.Errorf("functional case mismatch(es): %s", strings.Join(mismatches, "; "))
	}
	return nil
}

func (r *runner) courseVersion(ctx context.Context, id int) (code string, quota, enrolled, version int, err error) {
	err = r.pool.QueryRow(ctx, "SELECT code, quota, enrolled_count, version FROM courses WHERE id=$1", id).Scan(&code, &quota, &enrolled, &version)
	return
}

func responseVersion(got result) (int, error) {
	if got.err != nil {
		return 0, got.err
	}
	var envelope struct {
		Data struct {
			Version int `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(got.body, &envelope); err != nil {
		return 0, err
	}
	if envelope.Data.Version == 0 {
		return 0, fmt.Errorf("response tidak berisi data.version")
	}
	return envelope.Data.Version, nil
}

func (r *runner) hasEnrollment(ctx context.Context, userID, courseID int) bool {
	var exists bool
	err := r.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM enrollments WHERE user_id=$1 AND course_id=$2)", userID, courseID).Scan(&exists)
	return err == nil && exists
}

func checkHealth(baseURL string) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/api/v1/health")
	if err != nil {
		fatalf("health request: %v", err)
	}
	defer resp.Body.Close()
	fmt.Printf("health status=%d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		fatalf("health endpoint returned %d, want 200", resp.StatusCode)
	}
}

func checkLogs(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fatalf("read log %s: %v", path, err)
	}
	text := strings.ToLower(string(data))
	fmt.Printf("log=%s\nWARN request_rejected=%d\nCHECK constraint mentions=%d\ncontext deadline mentions=%d\n",
		path, strings.Count(text, `"level":"WARN","msg":"request_rejected"`),
		strings.Count(text, "courses_enrolled_within_quota"), strings.Count(text, "context deadline exceeded"))
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}
