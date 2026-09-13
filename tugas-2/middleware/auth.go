package middleware

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"api-students/helper"
)

func RequireAuth(jwtManager *helper.JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")

		if authHeader == "" {
			c.Set("WWW-Authenticate", `Bearer`)
			return helper.Fail(
				c,
				fiber.StatusUnauthorized,
				"access token diperlukan",
			)
		}

		parts := strings.Fields(authHeader)

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Set("WWW-Authenticate", `Bearer`)
			return helper.Fail(
				c,
				fiber.StatusUnauthorized,
				"format access token tidak valid",
			)
		}

		authUser, err := jwtManager.ParseAccessToken(parts[1])
		if err != nil {
			c.Set("WWW-Authenticate", `Bearer`)

			if errors.Is(err, helper.ErrExpiredToken) {
				return helper.Fail(
					c,
					fiber.StatusUnauthorized,
					"access token kedaluwarsa",
				)
			}

			return helper.Fail(
				c,
				fiber.StatusUnauthorized,
				"access token tidak valid",
			)
		}

		// Simpan identitas user untuk handler/service berikutnya.
		c.Locals("user_id", authUser.UserID)
		c.Locals("username", authUser.Username)
		c.Locals("role", authUser.Role)

		return c.Next()
	}
}
