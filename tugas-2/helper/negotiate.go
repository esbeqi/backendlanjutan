package helper

import (
	"strings"

	"api-students/app/model"

	"github.com/gofiber/fiber/v2"
)

// NegotiateFormat menentukan format response berdasarkan
// header Accept dari client.
//
// Format yang didukung:
//   - application/json
//   - text/csv
//   - */*
//
// Jika Accept kosong atau */*, JSON digunakan sebagai
// format bawaan.
func NegotiateFormat(c *fiber.Ctx) (string, bool) {
	accept := strings.TrimSpace(c.Get("Accept"))

	// Tidak ada Accept atau wildcard berarti
	// gunakan format bawaan, yaitu JSON.
	if accept == "" || accept == "*/*" {
		return "json", true
	}

	// Accept dapat berisi beberapa media type,
	// contoh:
	// application/json, text/csv;q=0.8
	//
	// Untuk kebutuhan praktikum, media type pertama
	// digunakan sebagai format yang diminta.
	mediaType := strings.TrimSpace(
		strings.Split(accept, ",")[0],
	)

	// Buang parameter seperti ;q=0.9.
	mediaType = strings.TrimSpace(
		strings.Split(mediaType, ";")[0],
	)

	switch strings.ToLower(mediaType) {

	case "application/json":
		return "json", true

	case "text/csv":
		return "csv", true

	default:
		return "", false
	}
}

// NotAcceptable mengembalikan HTTP 406
// jika format response yang diminta client
// tidak didukung oleh API.
func NotAcceptable(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotAcceptable).JSON(model.WebResponse{
		Success: false,
		Message: "format response tidak didukung",
	})
}
