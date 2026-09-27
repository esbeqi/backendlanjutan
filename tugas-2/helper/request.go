package helper

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"api-students/app/model"

	"github.com/gofiber/fiber/v2"
)

func RequestContext(c *fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.UserContext(), 5*time.Second)
}

func ParamID(c *fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, false
	}

	return id, true
}

var allowedSort = map[string]bool{
	"id":        true,
	"nim":       true,
	"name":      true,
	"grade":     true,
	"is_active": true,
}

func ParseListQuery(c *fiber.Ctx) model.ListQuery {

	q := model.ListQuery{
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 10),
		Search: strings.TrimSpace(c.Query("search")),
		Sort:   c.Query("sort", "id"),
		Order:  strings.ToLower(c.Query("order", "asc")),
	}

	if q.Page < 1 {
		q.Page = 1
	}

	if q.Limit < 1 {
		q.Limit = 10
	}

	if q.Limit > 50 {
		q.Limit = 50
	}

	if !allowedSort[q.Sort] {
		q.Sort = "id"
	}

	if q.Order != "desc" {
		q.Order = "asc"
	}

	if raw := c.Query("is_active"); raw != "" {
		if v, err := strconv.ParseBool(raw); err == nil {
			q.IsActive = &v
		}
	}

	return q
}

// ParseCursorQuery membaca query parameter untuk cursor pagination.
func ParseCursorQuery(c *fiber.Ctx) (model.CursorQuery, error) {

	q := model.CursorQuery{
		Limit:  c.QueryInt("limit", 10),
		Search: strings.TrimSpace(c.Query("search")),
	}

	if q.Limit < 1 {
		q.Limit = 10
	}

	if q.Limit > 50 {
		q.Limit = 50
	}

	if raw := c.Query("is_active"); raw != "" {
		if v, err := strconv.ParseBool(raw); err == nil {
			q.IsActive = &v
		}
	}

	q.Cursor = strings.TrimSpace(c.Query("cursor"))

	// Halaman pertama tidak membutuhkan cursor.
	if q.Cursor == "" {
		return q, nil
	}

	// Decode cursor dari Base64 URL-safe.
	decoded, err := base64.RawURLEncoding.DecodeString(q.Cursor)
	if err != nil {
		return q, fmt.Errorf("cursor tidak valid")
	}

	// Format cursor:
	// created_at|id
	parts := strings.Split(string(decoded), "|")

	if len(parts) != 2 {
		return q, fmt.Errorf("cursor tidak valid")
	}

	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return q, fmt.Errorf("cursor tidak valid")
	}

	id, err := strconv.Atoi(parts[1])
	if err != nil || id < 1 {
		return q, fmt.Errorf("cursor tidak valid")
	}

	q.CursorCreatedAt = &createdAt
	q.CursorID = &id

	return q, nil
}

// EncodeCursor mengubah pasangan (created_at, id)
// menjadi cursor yang dapat digunakan pada request berikutnya.
func EncodeCursor(cursor model.Cursor) string {

	raw := cursor.CreatedAt.Format(time.RFC3339Nano) +
		"|" +
		strconv.Itoa(cursor.ID)

	return base64.RawURLEncoding.EncodeToString(
		[]byte(raw),
	)
}
