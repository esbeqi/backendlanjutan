package config

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"api-students/app/service"
	"api-students/helper"
	"api-students/middleware"
	"api-students/route"
)

// NewApp merakit seluruh aplikasi:
// - membuat instance Fiber
// - memasang middleware
// - mendaftarkan route
func NewApp(
	logger *slog.Logger,
	pool *pgxpool.Pool,
	studentService *service.StudentService,
) *fiber.App {

	app := fiber.New(fiber.Config{
		AppName:      GetEnv("APP_NAME", "Student REST API"),
		ErrorHandler: newErrorHandler(logger),
	})

	// Global middleware
	middleware.Register(app, logger)

	// Route
	route.Register(app, pool, studentService)

	// Fallback endpoint
	app.Use(func(c *fiber.Ctx) error {
		return helper.Fail(
			c,
			fiber.StatusNotFound,
			"endpoint tidak ditemukan",
		)
	})

	return app
}

// Error handler terakhir
func newErrorHandler(logger *slog.Logger) fiber.ErrorHandler {

	return func(c *fiber.Ctx, err error) error {

		status := fiber.StatusInternalServerError
		message := "terjadi error pada server"

		if e, ok := err.(*fiber.Error); ok {
			status = e.Code
			message = e.Message
		}

		logger.Error(
			"unhandled_error",
			slog.String("path", c.Path()),
			slog.Int("status", status),
			slog.String("error", err.Error()),
		)

		return helper.Fail(
			c,
			status,
			message,
		)
	}
}
