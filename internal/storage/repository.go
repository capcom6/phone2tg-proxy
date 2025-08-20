package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "p2tg:phone:"

var (
	ErrPhoneNumberNotFound = errors.New("phone number not found")
)

//nolint:iface // interface is required
type Repository interface {
	Store(ctx context.Context, phoneNumber string, telegramID int64) error
	Get(ctx context.Context, phoneNumber string) (int64, error)
	Delete(ctx context.Context, phoneNumber string) error
}

type repository struct {
	client *redis.Client
}

func newRepository(client *redis.Client) Repository {
	return &repository{
		client: client,
	}
}

func (r *repository) Store(ctx context.Context, phoneNumber string, telegramID int64) error {
	if err := r.client.Set(ctx, makeKey(phoneNumber), telegramID, 0).Err(); err != nil {
		return fmt.Errorf("repository: %w", err)
	}
	return nil
}

func (r *repository) Get(ctx context.Context, phoneNumber string) (int64, error) {
	result, err := r.client.Get(ctx, makeKey(phoneNumber)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, ErrPhoneNumberNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("repository: %w", err)
	}

	return result, nil
}

func (r *repository) Delete(ctx context.Context, phoneNumber string) error {
	if err := r.client.Del(ctx, makeKey(phoneNumber)).Err(); err != nil {
		return fmt.Errorf("repository: %w", err)
	}
	return nil
}

func makeKey(k string) string { return keyPrefix + k }
