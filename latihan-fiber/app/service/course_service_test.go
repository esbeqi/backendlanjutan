package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/model"
	"latihan-fiber/helper"
)

func TestSwapSameCourseReturnsUnprocessableValidation(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			var appErr *helper.AppError
			if !errors.As(err, &appErr) {
				return c.SendStatus(fiber.StatusInternalServerError)
			}
			return c.Status(appErr.Status).JSON(model.ErrorResponse{
				Success: false,
				Code:    appErr.Code,
				Message: appErr.Message,
				Fields:  appErr.Fields,
			})
		},
	})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(helper.LocalsAuthUser, model.AuthUser{UserID: 1})
		return c.Next()
	})
	app.Post("/enrollments/swap", (*CourseService)(nil).Swap)

	req := httptest.NewRequest(http.MethodPost, "/enrollments/swap", strings.NewReader(`{"from_course_id":101,"to_course_id":101}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusUnprocessableEntity)
	}

	var body model.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != helper.CodeValidation {
		t.Errorf("code = %q, want %q", body.Code, helper.CodeValidation)
	}
	if body.Fields["to_course_id"] == "" {
		t.Errorf("fields[to_course_id] = %q, want validation detail", body.Fields["to_course_id"])
	}
}
