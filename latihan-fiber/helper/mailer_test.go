package helper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"latihan-fiber/app/model"
)

func TestLogMailerFailsEveryThirdAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "mail.log")
	mailer, err := NewLogMailer(path, 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		err := mailer.Send(context.Background(), model.Message{To: "student@example.invalid.local", Subject: "probe"})
		if attempt == 3 {
			if !errors.Is(err, ErrTemporaryDelivery) {
				t.Fatalf("attempt 3 error = %v, want temporary delivery failure", err)
			}
		} else if err != nil {
			t.Fatalf("attempt %d unexpectedly failed: %v", attempt, err)
		}
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(strings.Split(strings.TrimSpace(string(contents)), "\n")); got != 2 {
		t.Fatalf("mail log lines=%d, want 2 successful sends", got)
	}
}

func TestLogMailerStopsOnCancelledContext(t *testing.T) {
	mailer, err := NewLogMailer(filepath.Join(t.TempDir(), "mail.log"), time.Second, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := mailer.Send(ctx, model.Message{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled send error = %v, want context canceled", err)
	}
}
