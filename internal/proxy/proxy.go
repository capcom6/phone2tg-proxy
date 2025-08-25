package proxy

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		s.logger.Error("send", zap.String("phone_number", maskPhoneNumber(phoneNumber)), zap.Error(err))
		return 0, fmt.Errorf("send: %w", err)
	}

	return msg.ID, nil
}

const digitsToKeep = 4

func maskPhoneNumber(phone string) string {
	// Handle empty string and very short numbers
	if len(phone) == 0 {
		return ""
	}

	if len(phone) <= digitsToKeep {
		// For short numbers, mask all but the last digit if possible
		if len(phone) == 1 {
			return "*"
		}
		return "*" + phone[1:]
	}

	// Create a mask with the same length as the original number
	// keeping only the last 'digitsToKeep' digits visible
	masked := strings.Repeat("*", len(phone)-digitsToKeep)
	return masked + phone[len(phone)-digitsToKeep:]
}
