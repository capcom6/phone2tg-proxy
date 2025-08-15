package jsonify

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func New() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err //nolint:wrapcheck // already wrapped
		}

		contentType := string(c.Response().Header.ContentType())
		if strings.HasPrefix(contentType, "application/json") {
			return nil
		}

		body := c.Response().Body()

		if c.Response().StatusCode() < fiber.StatusBadRequest {
			if err := c.JSON(body); err != nil {
				// Fallback to string representation if JSON serialization fails
				return c.Status(fiber.StatusInternalServerError).JSON(fmt.Sprintf("failed to jsonify response: %s", err.Error()))
			}
			return nil
		}

		return fiber.NewError(c.Response().StatusCode(), string(body))
	}
}
