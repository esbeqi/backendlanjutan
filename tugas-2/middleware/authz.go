package middleware

import (
	"api-students/helper"

	"github.com/gofiber/fiber/v2"
)

func RequirePermission(
	perms *helper.PermissionSet,
	permission string,
) fiber.Handler {

	return func(c *fiber.Ctx) error {

		role, ok := c.Locals("role").(string)
		if !ok || role == "" {
			return helper.Fail(
				c,
				fiber.StatusUnauthorized,
				"identitas user tidak ditemukan",
			)
		}

		if !perms.Can(role, permission) {
			return helper.Fail(
				c,
				fiber.StatusForbidden,
				"tidak memiliki permission",
			)
		}

		c.Locals("permission_set", perms)

		return c.Next()
	}
}

// ProvidePermissionSet menyimpan PermissionSet ke context
// agar dapat digunakan oleh service untuk pengecekan ownership.
func ProvidePermissionSet(
	perms *helper.PermissionSet,
) fiber.Handler {

	return func(c *fiber.Ctx) error {
		c.Locals("permission_set", perms)
		return c.Next()
	}
}
