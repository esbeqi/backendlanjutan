package helper

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

var ErrPermanentFailure = errors.New("simulated permanent mail failure")

type Mailer struct {
	path           string
	delay          time.Duration
	failEvery      uint64
	permanentEvery uint64
	attempts       atomic.Uint64
}

func NewMailer() *Mailer {
	delay, _ := strconv.Atoi(os.Getenv("MAILER_DELAY_MS"))
	failEvery, _ := strconv.ParseUint(os.Getenv("MAILER_FAIL_EVERY"), 10, 64)
	permanentEvery, _ := strconv.ParseUint(os.Getenv("MAILER_PERMANENT_FAIL_EVERY"), 10, 64)
	return &Mailer{path: "logs/mail.log", delay: time.Duration(delay) * time.Millisecond, failEvery: failEvery, permanentEvery: permanentEvery}
}

func (m *Mailer) Send(to, subject, body string) error {
	n := m.attempts.Add(1)
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	if m.permanentEvery > 0 && n%m.permanentEvery == 0 {
		return fmt.Errorf("%w on attempt %d", ErrPermanentFailure, n)
	}
	if m.failEvery > 0 && n%m.failEvery == 0 {
		return fmt.Errorf("simulated temporary mail failure on attempt %d", n)
	}
	if err := os.MkdirAll("logs", 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(m.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "to=%q subject=%q body=%q\n", to, subject, body)
	return err
}
