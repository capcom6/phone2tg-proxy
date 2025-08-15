package middleware

import (
	"go.uber.org/zap"
	"gopkg.in/telebot.v4"
)

func Logger(logger *zap.Logger) telebot.MiddlewareFunc {
	return func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) error {
			chatID := int64(0)
			if ch := c.Chat(); ch != nil {
				chatID = ch.ID
			}
			fromID := int64(0)
			if u := c.Sender(); u != nil {
				fromID = u.ID
			}
			l := logger.With(
				zap.Int("update_id", c.Update().ID),
				zap.Int64("chat_id", chatID),
				zap.Int64("from_id", fromID),
			)
			l.Debug("update received")

			err := next(c)
			if err != nil {
				l.Error(
					"update processing error",
					zap.Any("update", c.Update()),
					zap.Error(err),
				)
			}

			return err
		}
	}
}
