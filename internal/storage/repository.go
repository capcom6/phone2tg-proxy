package storage

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const (
	keyPrefixPhone = "p2tg:phone:"
	keyPrefixTg    = "p2tg:tg:"
)

var (
	ErrPhoneNumberNotFound = errors.New("phone number not found")
)

type Repository interface {
	Store(ctx context.Context, phoneNumber string, telegramID int64) error
	Get(ctx context.Context, phoneNumber string) (int64, error)
	DeleteByPhoneNumber(ctx context.Context, phoneNumber string) error
	DeleteByTelegramID(ctx context.Context, telegramID int64) error
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
	if err := r.delete(ctx, phoneNumber, telegramID); err != nil {
		return fmt.Errorf("store: %w", err)
	}

	err := r.client.MSet(
		ctx,
		makePhoneKey(phoneNumber), telegramID,
		makeTgKey(telegramID), phoneNumber,
	).Err()
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}

	return nil
}

func (r *repository) Get(ctx context.Context, phoneNumber string) (int64, error) {
	result, err := r.client.Get(ctx, makePhoneKey(phoneNumber)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, ErrPhoneNumberNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get: %w", err)
	}

	return result, nil
}

func (r *repository) DeleteByPhoneNumber(ctx context.Context, phoneNumber string) error {
	telegramID, err := r.Get(ctx, phoneNumber)
	if errors.Is(err, ErrPhoneNumberNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	return r.delete(ctx, phoneNumber, telegramID)
}

func (r *repository) DeleteByTelegramID(ctx context.Context, telegramID int64) error {
	phoneNumber, err := r.client.Get(ctx, makeTgKey(telegramID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("delete by telegram id: %w", err)
	}

	if err := r.delete(ctx, phoneNumber, telegramID); err != nil {
		return fmt.Errorf("delete by telegram id: %w", err)
	}

	return nil
}

func (r *repository) delete(ctx context.Context, phoneNumber string, telegramID int64) error {
	if err := r.client.Del(ctx, makePhoneKey(phoneNumber), makeTgKey(telegramID)).Err(); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

func makePhoneKey(k string) string { return keyPrefixPhone + k }
func makeTgKey(k int64) string     { return keyPrefixTg + strconv.FormatInt(k, 10) }
