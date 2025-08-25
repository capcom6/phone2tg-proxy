package redis

import (
	"context"
	"fmt"

	"github.com/capcom6/phone2tg-proxy/pkg/fxutil"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"redis",
		fxutil.WithNamedLogger("redis"),
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
