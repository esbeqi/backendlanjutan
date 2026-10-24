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
	studentPortfolioService *service.StudentPortfolioService,
	supervisorService *service.SupervisorService,
	authService *service.AuthService,
	jwtManager *helper.JWTManager,
	perms *helper.PermissionSet,
) {

	api := app.Group("/api/v1")

	// Public endpoint
	api.Get("/health", healthCheck(pool))

	// Register the static statistics path before parameterized student routes.
	portfolios := api.Group(
		"/portfolios",
		middleware.RequireAuth(jwtManager),
		middleware.ProvidePermissionSet(perms),
	)
	portfolios.Get("/stats/technologies", studentPortfolioService.TechnologyStats)

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
		middleware.ProvidePermissionSet(perms),
	)

	// Memerlukan permission student:list
	students.Get(
		"/",
		middleware.RequirePermission(perms, "student:list"),
		studentService.List,
	)

	students.Get("/:id/portfolio", studentPortfolioService.Get)
	students.Put("/:id/portfolio", studentPortfolioService.Replace)
	students.Post("/:id/portfolio/projects", studentPortfolioService.AddProject)
	students.Delete("/:id/portfolio/projects/:projectId", studentPortfolioService.DeleteProject)

	// Ownership / student:read:any ditangani di service
	students.Get("/:id", studentService.Get)

	// Memerlukan permission student:create
	students.Post(
		"/",
		middleware.RequirePermission(perms, "student:create"),
		studentService.Create,
	)

	// Ownership / student:update:any ditangani di service
	students.Put("/:id", studentService.Replace)
	students.Patch("/:id", studentService.Patch)

	// Memerlukan permission student:delete
	students.Delete(
		"/:id",
		middleware.RequirePermission(perms, "student:delete"),
		studentService.Delete,
	)

	supervisors := api.Group(
		"/supervisors",
		middleware.RequireJSON,
		middleware.RequireAuth(jwtManager),
		middleware.ProvidePermissionSet(perms),
	)
	supervisors.Post(
		"/:id/assign",
		middleware.RequirePermission(perms, "supervision:manage"),
		supervisorService.Assign,
	)
	supervisors.Patch(
		"/:id",
		middleware.RequirePermission(perms, "supervision:manage"),
		supervisorService.Patch,
	)

	supervisions := api.Group(
		"/supervisions",
		middleware.RequireJSON,
		middleware.RequireAuth(jwtManager),
		middleware.ProvidePermissionSet(perms),
	)
	supervisions.Post(
		"/switch",
		middleware.RequirePermission(perms, "supervision:manage"),
		supervisorService.Switch,
	)
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
