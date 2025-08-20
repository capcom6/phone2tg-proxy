package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

var (
	ErrInvalidPhoneNumber = errors.New("invalid phone number")
)

//nolint:iface // interface is required
type Service interface {
	Store(ctx context.Context, phoneNumber string, telegramID int64) error
	Get(ctx context.Context, phoneNumber string) (int64, error)
	Delete(ctx context.Context, phoneNumber string) error
}

type service struct {
	c Config
	r Repository
	l *zap.Logger
}

func New(c Config, r Repository, l *zap.Logger) Service {
	return &service{
		c: c,
		r: r,
		l: l,
	}
}

func normalizePhoneNumber(phoneNumber string) string {
	var b strings.Builder
	b.Grow(len(phoneNumber))
	for _, r := range phoneNumber {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func hmacPhone(phoneNumber string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(phoneNumber))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *service) Store(ctx context.Context, phoneNumber string, telegramID int64) error {
	hashed, err := s.preparePhoneNumber(phoneNumber)
	if err != nil {
		return fmt.Errorf("service: %w", err)
	}

	if err := s.r.Store(ctx, hashed, telegramID); err != nil {
		return fmt.Errorf("service: %w", err)
	}

	return nil
}

func (s *service) Get(ctx context.Context, phoneNumber string) (int64, error) {
	hashed, err := s.preparePhoneNumber(phoneNumber)
	if err != nil {
		return 0, fmt.Errorf("service: %w", err)
	}

	id, err := s.r.Get(ctx, hashed)
	if err != nil {
		return 0, fmt.Errorf("service: %w", err)
	}

	return id, nil
}

func (s *service) Delete(ctx context.Context, phoneNumber string) error {
	hashed, err := s.preparePhoneNumber(phoneNumber)
	if err != nil {
		return fmt.Errorf("service: %w", err)
	}

	if err := s.r.Delete(ctx, hashed); err != nil {
		return fmt.Errorf("service: %w", err)
	}

	return nil
}

func (s *service) preparePhoneNumber(phoneNumber string) (string, error) {
	normalized := normalizePhoneNumber(phoneNumber)
	if normalized == "" {
		return "", ErrInvalidPhoneNumber
	}

	if len(s.c.Secret) == 0 {
		s.l.Warn("secret is empty")
	}

	return hmacPhone(normalized, s.c.Secret), nil
}
