package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// StatusClientClosedRequest is the HTTP status code for client closed request errors.
const StatusClientClosedRequest = 499

// ErrorFormatter formats an error and HTTP status code into a JSON-serializable payload.
type ErrorFormatter func(err error, code int) any

// NewViewsErrorHandler creates a fiber.ErrorHandler that renders the given template with the error message
// and status code. If rendering fails, it falls back to sending the error message as a plain-text response.
// The logger is used to log errors that occur while rendering the view.
// The layouts are passed to the Render method as is.
func NewViewsErrorHandler(logger *zap.Logger, template string, layouts ...string) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := preHandleError(err, logger)

		msg := makeMessage(err, code)

		if rerr := c.Status(code).Render(template, fiber.Map{"error": msg, "code": code}, layouts...); rerr != nil {
			logger.Error("failed to render error view", zap.Error(rerr))
			return c.Status(code).SendString(msg)
		}

		return nil
	}
}

// NewJSONErrorHandler returns a fiber.ErrorHandler that formats the given error and HTTP status code
// using the NewErrorResponse function, and sends the result as a JSON response.
func NewJSONErrorHandler(logger *zap.Logger) fiber.ErrorHandler {
	return NewCustomJSONErrorHandler(logger, nil)
}

// NewCustomJSONErrorHandler returns a fiber.ErrorHandler that formats the given error and HTTP status code
// using the provided ErrorFormatter, and sends the result as a JSON response. If the formatter is nil, it falls
// back to using NewErrorResponse to format the error.
func NewCustomJSONErrorHandler(logger *zap.Logger, formatter ErrorFormatter) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := preHandleError(err, logger)

		if formatter != nil {
			return c.Status(code).JSON(formatter(err, code))
		}

		return c.Status(code).JSON(NewErrorResponse(makeMessage(err, code), code, nil))
	}
}

func preHandleError(err error, logger *zap.Logger) int {
	code := fiber.StatusInternalServerError

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
	}

	if errors.Is(err, context.Canceled) {
		// Non-standard but widely used; switch to fiber.StatusRequestTimeout if you prefer only standard codes.
		code = StatusClientClosedRequest
	} else if errors.Is(err, context.DeadlineExceeded) {
		code = fiber.StatusRequestTimeout
	}

	if code >= fiber.StatusInternalServerError {
		logger.Error("http handler error", zap.Error(err), zap.Int("code", code))
	}

	return code
}

func makeMessage(err error, code int) string {
	msg := err.Error()

	// Normalize select 4xx coming from context sentinels.
	switch code {
	case StatusClientClosedRequest:
		return "client closed request"
	case fiber.StatusRequestTimeout:
		if m := http.StatusText(code); m != "" {
			return m
		}
		return "request timeout"
	}

	if code >= fiber.StatusInternalServerError {
		if m := http.StatusText(code); m != "" {
			return m
		}
		return "internal server error"
	}
	return msg
}
