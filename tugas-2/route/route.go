package route

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"api-students/app/service"
	"api-students/helper"
	"api-students/middleware"
)

func Register(
	app *fiber.App,
	pool *pgxpool.Pool,
	studentService *service.StudentService,
	authService *service.AuthService,
	jwtManager *helper.JWTManager,
) {

	api := app.Group("/api/v1")

	// Public endpoint
	api.Get("/health", healthCheck(pool))

	// Authentication
	auth := api.Group("/auth")

	auth.Post("/register", authService.Register)
	auth.Post(
		"/login",
		middleware.LoginRateLimit,
		authService.Login,
	)
	auth.Post("/refresh", authService.Refresh)
	auth.Post("/logout", authService.Logout)

	auth.Get(
		"/me",
		middleware.RequireAuth(jwtManager),
		authService.Me,
	)

	// Protected student endpoints
	students := api.Group(
		"/students",
		middleware.RequireJSON,
		middleware.RequireAuth(jwtManager),
	)

	students.Get("/", studentService.List)
	students.Get("/:id", studentService.Get)
	students.Post("/", studentService.Create)
	students.Put("/:id", studentService.Replace)
	students.Patch("/:id", studentService.Patch)
	students.Delete("/:id", studentService.Delete)
}

func healthCheck(pool *pgxpool.Pool) fiber.Handler {

	return func(c *fiber.Ctx) error {

		ctx, cancel := context.WithTimeout(
			c.UserContext(),
			2*time.Second,
		)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {

			return helper.Fail(
				c,
				fiber.StatusServiceUnavailable,
				"database tidak dapat dihubungi",
			)
		}

		return helper.Success(
			c,
			fiber.StatusOK,
			"server dan database berjalan",
			nil,
		)
	}
}
