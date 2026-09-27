package helper

import (
	"fmt"
	"strings"
	"time"

	"api-students/app/model"

	"github.com/gofiber/fiber/v2"
)

// Success mengembalikan response sukses umum dalam format JSON.
func Success(
	c *fiber.Ctx,
	status int,
	message string,
	data any,
) error {
	return c.Status(status).JSON(model.WebResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// SuccessList mengembalikan response untuk
// pagination berbasis offset.
func SuccessList(
	c *fiber.Ctx,
	message string,
	data any,
	meta *model.Meta,
) error {
	return c.Status(fiber.StatusOK).JSON(model.WebResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// SuccessCursorList mengembalikan response untuk
// pagination berbasis cursor.
func SuccessCursorList(
	c *fiber.Ctx,
	message string,
	data any,
	meta *model.CursorMeta,
) error {
	return c.Status(fiber.StatusOK).JSON(
		model.WebResponse{
			Success: true,
			Message: message,
			Data: map[string]any{
				"items": data,
				"meta":  meta,
			},
		},
	)
}

// SuccessCSV mengembalikan daftar mahasiswa
// dalam format CSV.
func SuccessCSV(
	c *fiber.Ctx,
	students []model.Student,
) error {

	var b strings.Builder

	// Header CSV.
	b.WriteString(
		"id,nim,name,grade,is_active,owner_id,created_at\n",
	)

	// Data mahasiswa.
	for _, s := range students {
		b.WriteString(
			fmt.Sprintf(
				"%d,%s,%s,%d,%t,%d,%s\n",
				s.ID,
				s.NIM,
				s.Name,
				s.Grade,
				s.IsActive,
				s.OwnerID,
				s.CreatedAt.Format(time.RFC3339Nano),
			),
		)
	}

	c.Set(
		"Content-Type",
		"text/csv; charset=utf-8",
	)

	c.Set(
		"Content-Disposition",
		`attachment; filename="students.csv"`,
	)

	return c.Status(fiber.StatusOK).SendString(
		b.String(),
	)
}

// Created mengembalikan response 201 Created.
func Created(
	c *fiber.Ctx,
	message string,
	data any,
	location string,
) error {
	c.Set("Location", location)

	return c.Status(fiber.StatusCreated).JSON(
		model.WebResponse{
			Success: true,
			Message: message,
			Data:    data,
		},
	)
}

// NoContent mengembalikan response 204 No Content.
func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// Fail mengembalikan response error.
func Fail(
	c *fiber.Ctx,
	status int,
	message string,
) error {
	return c.Status(status).JSON(
		model.WebResponse{
			Success: false,
			Message: message,
		},
	)
}

// FailValidation mengembalikan response
// ketika validasi request gagal.
func FailValidation(
	c *fiber.Ctx,
	errs map[string]string,
) error {
	return c.Status(
		fiber.StatusUnprocessableEntity,
	).JSON(
		model.WebResponse{
			Success: false,
			Message: "validasi gagal",
			Errors:  errs,
		},
	)
}
