package proxy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/go-core-fx/telegofx"
	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

var (
	ErrPhoneNumberNotFound = errors.New("phone number not found")
)

type Service interface {
	Send(ctx context.Context, phoneNumber string, message string) (int, error)
}

// sendTimeout bounds one Telegram send so a stalled Telegram or SOCKS5 request
// cannot pin the caller's goroutine for the transport's lifetime.
const sendTimeout = 10 * time.Second

type service struct {
	storage storage.Service
	bot     *telegofx.Bot

	logger *zap.Logger
}

func New(storage storage.Service, bot *telegofx.Bot, logger *zap.Logger) Service {
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

	// The send gets its own budget, detached from the caller: the caller's
	// context bounds only the storage lookup above.
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sendTimeout)
	defer cancel()

	msg, err := s.bot.SendMessage(sendCtx, &telego.SendMessageParams{
		ChatID: telego.ChatID{ID: telegramID},
		Text:   message,
	})
	if err != nil {
		s.logger.Error("send", zap.String("phone_number", maskPhoneNumber(phoneNumber)), zap.Error(err))
		return 0, fmt.Errorf("send: %w", err)
	}

	return msg.MessageID, nil
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
