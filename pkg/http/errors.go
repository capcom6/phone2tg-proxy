package http

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type ErrorFormatter func(err error, code int) any

func NewViewsErrorHandler(logger *zap.Logger, template string, layouts ...string) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := preHandleError(err, logger)

		if rerr := c.Status(code).Render(template, fiber.Map{"error": err.Error(), "code": code}, layouts...); rerr != nil {
			logger.Error("failed to render error view", zap.Error(rerr), zap.Int("code", code))
			return c.Status(code).SendString(err.Error())
		}

		return nil
	}
}

func NewJSONErrorHandler(logger *zap.Logger) fiber.ErrorHandler {
	return NewCustomJSONErrorHandler(logger, nil)
}

func NewCustomJSONErrorHandler(logger *zap.Logger, formatter ErrorFormatter) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := preHandleError(err, logger)

		if formatter != nil {
			return c.Status(code).JSON(formatter(err, code))
		}

		msg := err.Error()
		if code >= fiber.StatusInternalServerError {
			msg = "internal server error"
		}
		return c.Status(code).JSON(NewErrorResponse(msg, code, nil))
	}
}

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
