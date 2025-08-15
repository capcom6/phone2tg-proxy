package middleware

import (
	"go.uber.org/zap"
	"gopkg.in/telebot.v4"
)

func Logger(logger *zap.Logger) telebot.MiddlewareFunc {
	return func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) error {
			logger.Info(
				"update received",
				zap.Int("update_id", c.Update().ID),
			)

			err := next(c)
			if err != nil {
				logger.Error(
					"update processing error",
					zap.Any("update", c.Update()),
					zap.Error(err),
				)
			}

			return err
		}
	}
}
