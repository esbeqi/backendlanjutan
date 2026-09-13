package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"

	"api-students/helper"
)

type loginAttempt struct {
	count       int
	windowStart time.Time
}

var (
	loginAttempts = make(map[string]loginAttempt)
	loginMu       sync.Mutex
)

func LoginRateLimit(c *fiber.Ctx) error {
	ip := c.IP()
	now := time.Now()

	loginMu.Lock()

	attempt, exists := loginAttempts[ip]

	if !exists || now.Sub(attempt.windowStart) >= time.Minute {
		attempt = loginAttempt{
			count:       0,
			windowStart: now,
		}
	}

	if attempt.count >= 5 {
		retryAfter := int(time.Until(attempt.windowStart.Add(time.Minute)).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}

		loginMu.Unlock()

		c.Set("Retry-After", strconv.Itoa(retryAfter))

		return helper.Fail(
			c,
			fiber.StatusTooManyRequests,
			"terlalu banyak percobaan login",
		)
	}

	loginMu.Unlock()

	err := c.Next()

	if c.Response().StatusCode() == fiber.StatusUnauthorized {
		loginMu.Lock()

		attempt.count++
		loginAttempts[ip] = attempt

		loginMu.Unlock()
	} else if c.Response().StatusCode() == fiber.StatusOK {
		loginMu.Lock()
		delete(loginAttempts, ip)
		loginMu.Unlock()
	}

	return err
}
