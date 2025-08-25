package proxy

import (
	"context"
	"errors"
	"fmt"

	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"go.uber.org/zap"
	"gopkg.in/telebot.v4"
)

var (
	ErrPhoneNumberNotFound = errors.New("phone number not found")
)

type Service interface {
	Send(ctx context.Context, phoneNumber string, message string) (int, error)
}

type service struct {
	storage storage.Service
	bot     *telebot.Bot

	logger *zap.Logger
}

func New(storage storage.Service, bot *telebot.Bot, logger *zap.Logger) Service {
	return &service{
		storage: storage,
		bot:     bot,
		logger:  logger,
	}
}

// Send implements Service.
func (s *service) Send(ctx context.Context, phoneNumber string, message string) (int, error) {
	telegramID, err := s.storage.Get(ctx, phoneNumber)
	if errors.Is(err, storage.ErrPhoneNumberNotFound) {
		return 0, ErrPhoneNumberNotFound
	}

	if err != nil {
		return 0, fmt.Errorf("send: %w", err)
	}

	msg, err := s.bot.Send(telebot.ChatID(telegramID), message)
	if err != nil {
		return 0, fmt.Errorf("send: %w", err)
	}

	return msg.ID, nil
}
