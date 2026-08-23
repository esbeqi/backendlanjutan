package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
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
			return fail(c, fiber.StatusUnsupportedMediaType,
				"Content-Type harus application/json")
		}
	}

	return c.Next()
}

func main() {

	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Student REST API")
	})

	api := app.Group("/api/v1")

	s := api.Group("/students", requireJSON)

	s.Get("/", listStudents)
	s.Get("/:id", getStudent)
	s.Post("/", createStudent)
	s.Put("/:id", replaceStudent)
	s.Patch("/:id", patchStudent)
	s.Delete("/:id", deleteStudent)

	app.Use(func(c *fiber.Ctx) error {
		return fail(c, fiber.StatusNotFound,
			"endpoint tidak ditemukan")
	})

	fmt.Println("Server berjalan di http://localhost:3000")

	log.Fatal(app.Listen(":3000"))
}
