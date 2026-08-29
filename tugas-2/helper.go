package main

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"api-students/app/model"
)

// Response 200 OK
func ok(c *fiber.Ctx, message string, data any) error {
	return c.Status(fiber.StatusOK).JSON(model.WebResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Response 200 OK untuk data list + pagination
func okList(c *fiber.Ctx, message string, data any, meta *model.Meta) error {
	return c.Status(fiber.StatusOK).JSON(model.WebResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// Response 201 Created
func created(c *fiber.Ctx, message string, data any, location string) error {
	c.Set("Location", location)

	return c.Status(fiber.StatusCreated).JSON(model.WebResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Response 204 No Content
func noContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// Response gagal biasa
func fail(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(model.WebResponse{
		Success: false,
		Message: message,
	})
}

// Response validasi
func failValidation(c *fiber.Ctx, errs map[string]string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(model.WebResponse{
		Success: false,
		Message: "validasi gagal",
		Errors:  errs,
	})
}

// Field yang boleh dipakai untuk sorting
var allowedSort = map[string]bool{
	"id":        true,
	"nim":       true,
	"name":      true,
	"grade":     true,
	"is_active": true,
}

// Membaca query string endpoint GET /students
func parseListQuery(c *fiber.Ctx) model.ListQuery {
	q := model.ListQuery{
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 10),
		Search: strings.TrimSpace(c.Query("search")),
		Sort:   c.Query("sort", "id"),
		Order:  strings.ToLower(c.Query("order", "asc")),
	}

	// Minimal page = 1
	if q.Page < 1 {
		q.Page = 1
	}

	// Default limit
	if q.Limit < 1 {
		q.Limit = 10
	}

	// Maksimal limit
	if q.Limit > 50 {
		q.Limit = 50
	}

	// Sort hanya boleh berdasarkan whitelist
	if !allowedSort[q.Sort] {
		q.Sort = "id"
	}

	// Order hanya asc atau desc
	if q.Order != "desc" {
		q.Order = "asc"
	}

	// Filter is_active
	if raw := c.Query("is_active"); raw != "" {
		if v, err := strconv.ParseBool(raw); err == nil {
			q.IsActive = &v
		}
	}

	return q
}
