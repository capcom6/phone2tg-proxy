package http

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func preHandleError(err error, logger *zap.Logger) int {
	code := fiber.StatusInternalServerError

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
	}

	if code >= fiber.StatusInternalServerError {
		logger.Error("An error occurred", zap.Error(err))
	}

	return code
}

func NewViewsErrorHandler(logger *zap.Logger, template string, layouts ...string) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := preHandleError(err, logger)

		return c.Status(code).Render(template, fiber.Map{"error": err.Error(), "code": code}, layouts...)
	}
}

func NewJSONErrorHandler(logger *zap.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := preHandleError(err, logger)

		errorResponse := NewErrorResponse(err.Error(), code, nil)

		return c.Status(code).JSON(errorResponse)
	}
}
