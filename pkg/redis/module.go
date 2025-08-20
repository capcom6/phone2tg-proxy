package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"redis",
		fx.Decorate(func(log *zap.Logger) *zap.Logger {
			return log.Named("redis")
		}),
		fx.Provide(New),
		fx.Invoke(func(lc fx.Lifecycle, client *redis.Client) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					if err := client.Ping(ctx).Err(); err != nil {
						return fmt.Errorf("redis ping: %w", err)
					}
					return nil
				},
				OnStop: func(_ context.Context) error {
					return client.Close()
				},
			})
		}),
	)
}
