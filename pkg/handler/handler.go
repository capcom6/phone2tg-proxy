package handler

import (
	"fmt"

	"github.com/capcom6/phone2tg-proxy/pkg/validator"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type Base struct {
	Validator *validator.Validate
	Logger    *zap.Logger
}

func (c *Base) BodyParserValidator(ctx *fiber.Ctx, out interface{}) error {
	if err := ctx.BodyParser(out); err != nil {
		c.Logger.Error("failed to parse request", zap.Error(err))
		return fmt.Errorf("failed to parse request: %w", err)
	}

	if err := c.Validator.Var(out, "required,dive"); err != nil {
		c.Logger.Error("failed to validate request", zap.Error(err))
		return fmt.Errorf("failed to validate request: %w", err)
	}

	if v, ok := out.(validator.Validatable); ok {
		if err := v.Validate(); err != nil {
			c.Logger.Error("failed to validate request", zap.Error(err))
			return fmt.Errorf("failed to validate request: %w", err)
		}
	}

	return nil
}
