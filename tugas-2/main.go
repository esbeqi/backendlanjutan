package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"api-students/app/repository"
	"api-students/config"
	"api-students/database"
)

var bodyMethods = map[string]bool{
	fiber.MethodPost:  true,
	fiber.MethodPut:   true,
	fiber.MethodPatch: true,
}

// Middleware untuk memastikan request yang memiliki body menggunakan JSON
func requireJSON(c *fiber.Ctx) error {
	if bodyMethods[c.Method()] {

		contentType := c.Get("Content-Type")

		if !strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			return fail(
				c,
				fiber.StatusUnsupportedMediaType,
				"Content-Type harus application/json",
			)
		}
	}

	return c.Next()
}

func main() {

	config.LoadEnv()

	pool, err := database.NewPool(context.Background())
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	// Repository & Handler
	studentRepo := repository.NewStudentRepository(pool)
	studentHandler := NewStudentHandler(studentRepo)

	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Student REST API")
	})

	api := app.Group("/api/v1")

	// Health Check
	api.Get("/health", func(c *fiber.Ctx) error {

		ctx, cancel := context.WithTimeout(
			c.UserContext(),
			2*time.Second,
		)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			return fail(
				c,
				fiber.StatusServiceUnavailable,
				"database tidak dapat dihubungi",
			)
		}

		return ok(
			c,
			"server dan database berjalan",
			nil,
		)
	})

	// Student API
	s := api.Group("/students", requireJSON)

	s.Get("/", studentHandler.List)
	s.Get("/:id", studentHandler.Get)
	s.Post("/", studentHandler.Create)
	s.Put("/:id", studentHandler.Replace)
	s.Patch("/:id", studentHandler.Patch)
	s.Delete("/:id", studentHandler.Delete)

	app.Use(func(c *fiber.Ctx) error {
		return fail(
			c,
			fiber.StatusNotFound,
			"endpoint tidak ditemukan",
		)
	})

	port := config.GetEnv("APP_PORT", "3000")

	fmt.Printf(
		"Server berjalan di http://localhost:%s\n",
		port,
	)

	log.Fatal(app.Listen(":" + port))
}
