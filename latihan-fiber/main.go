package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"latihan-fiber/app/repository"
	"latihan-fiber/config"
	"latihan-fiber/database"
)

var bodyMethods = map[string]bool{
	fiber.MethodPost:  true,
	fiber.MethodPut:   true,
	fiber.MethodPatch: true,
}

// Middleware memastikan request yang memiliki body menggunakan JSON
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

	// 1. Konfigurasi
	config.LoadEnv()

	// 2. Koneksi basis data
	pool, err := database.NewPool(context.Background())
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	// 3. Perakitan: pool -> repository -> handler
	userRepository := repository.NewUserRepository(pool)
	userHandler := NewUserHandler(userRepository)

	// 4. Aplikasi
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("User REST API")
	})

	api := app.Group("/api/v1")

	api.Get("/health", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()

		// Kesehatan layanan kini ikut bergantung pada basis data.
		if err := pool.Ping(ctx); err != nil {
			return fail(
				c,
				fiber.StatusServiceUnavailable,
				"database tidak dapat dihubungi",
			)
		}

		return ok(c, "server dan database berjalan", nil)
	})

	u := api.Group("/users", requireJSON)

	u.Get("/", userHandler.List)
	u.Get("/:id", userHandler.Get)
	u.Post("/", userHandler.Create)
	u.Put("/:id", userHandler.Replace)
	u.Patch("/:id", userHandler.Patch)
	u.Delete("/:id", userHandler.Delete)

	app.Use(func(c *fiber.Ctx) error {
		return fail(
			c,
			fiber.StatusNotFound,
			"endpoint tidak ditemukan",
		)
	})

	port := config.GetEnv("APP_PORT", "3000")

	fmt.Printf("Server berjalan di http://localhost:%s\n", port)

	log.Fatal(app.Listen(":" + port))
}
