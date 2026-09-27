package helper

import "github.com/gofiber/fiber/v2"

// RequestID mengambil ID unik request yang dipasang
// oleh middleware requestid.
func RequestID(c *fiber.Ctx) string {
	requestID, _ := c.Locals("requestid").(string)
	return requestID
}
