package bot_test

// wiring_test.go proves the telegofx router wiring the bot registers on
// resolves OFFLINE through fx: construction only, no graph Start, no network.
//
// This is the acceptance-criteria wiring gate. The fx graph resolves
// *telegofx.Router from telegofx.Config plus []telego.BotOption with a fake
// API caller, which is the exact seam internal/bot/module.go invokes against.
// go build and go vet cannot see a missing provider; fx.New can.

import (
	"context"
	"testing"

	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/telegofx"
	"github.com/mymmrac/telego"
	ta "github.com/mymmrac/telego/telegoapi"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// fakeCaller answers every Bot API call without a network. It exists so the
// graph can be constructed with telego.WithAPICaller(fake) and never leaves
// the process.
type fakeCaller struct{}

// Call implements telegoapi.Caller.
func (fakeCaller) Call(_ context.Context, _ string, _ *ta.RequestData) (*ta.Response, error) {
	return &ta.Response{Ok: true, Error: nil, Result: nil}, nil
}

// TestWiring_TelegofxRouterResolvesOffline builds the acceptance-criteria
// graph: logger + telegofx with the router, a format-valid token, a fake API
// caller, and fx.Populate into *telegofx.Router. Only construction is
// asserted; Start is never called, so no polling begins.
func TestWiring_TelegofxRouterResolvesOffline(t *testing.T) {
	t.Parallel()

	var rt *telegofx.Router

	err := fx.New(
		logger.Module(),
		telegofx.Module(telegofx.WithRouter()),
		fx.Supply(telegofx.Config{
			Token:    offlineToken,
			Mode:     telegofx.ModePolling,
			ProxyURL: "",
		}),
		fx.Supply([]telego.BotOption{telego.WithAPICaller(fakeCaller{})}),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
		fx.Populate(&rt),
	).Err()
	if err != nil {
		t.Fatalf("telegofx router wiring must resolve offline: %v", err)
	}

	if rt == nil {
		t.Fatal("graph resolved but did not populate *telegofx.Router")
	}
}
