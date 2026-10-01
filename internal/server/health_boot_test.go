package server_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/capcom6/phone2tg-proxy/internal/proxy"
	"github.com/capcom6/phone2tg-proxy/internal/server"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/healthfx"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/validatorfx"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
)

// errStubSend is returned by the stub proxy, which exists so the graph can boot
// without Redis or Telegram.
var errStubSend = errors.New("stub proxy send")

// stubProxy satisfies proxy.Service without touching Redis or Telegram.
type stubProxy struct{}

func (stubProxy) Send(_ context.Context, _, _ string) (int, error) {
	return 0, errStubSend
}

// TestAppBootSmoke boots the server fx graph (Start -> Stop) and asserts the
// health routes are registered on the real fiber app.
func TestAppBootSmoke(t *testing.T) {
	var app *fiber.App

	fxApp := fx.New(
		logger.Module(),
		logger.WithFxDefaultLogger(),

		fiberfx.Module(),
		fx.Provide(func() fiberfx.Config {
			return fiberfx.Config{
				Address:     "127.0.0.1:0",
				ProxyHeader: "",
				Proxies:     []string{},
			}
		}),

		healthfx.Module(),
		fx.Provide(func() healthfx.Version {
			return healthfx.Version{
				Version:   "dev",
				ReleaseID: 0,
				BuildDate: "",
				GitCommit: "",
				GoVersion: "",
			}
		}),

		validatorfx.Module(),
		fx.Provide(func() proxy.Service { return stubProxy{} }),

		server.Module(),

		fx.Populate(&app),
	)

	if err := fxApp.Start(t.Context()); err != nil {
		t.Fatalf("start graph: %v", err)
	}

	t.Cleanup(func() {
		// t.Context() is already canceled once cleanups run, so use a fresh one.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := fxApp.Stop(ctx); err != nil {
			t.Errorf("stop graph: %v", err)
		}
	})

	if app == nil {
		t.Fatal("fiber app was not populated")
	}

	want := map[string]bool{
		"/health":         false,
		"/health/live":    false,
		"/health/ready":   false,
		"/health/startup": false,
	}
	for _, route := range app.GetRoutes() {
		if _, ok := want[route.Path]; ok && route.Method == http.MethodGet {
			want[route.Path] = true
		}
	}

	for path, found := range want {
		if !found {
			t.Errorf("GET %s is not registered", path)
		}
	}
}
