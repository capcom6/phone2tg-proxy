package internal_test

// app_test.go pins the production fx graph from internal/app.go: the exact
// module list, plus the one observable that motivates WithHealth - the
// aggregated health service must carry the telegram:api liveness check.
// go build and go vet cannot see a missing module option; CheckLiveness can.
//
// The graph is assembled here the same way graph_test.go mirrors
// internal/bot/module.go: module by module, construction only. Nothing is
// started, so no listener binds, no Redis connection opens and no polling
// begins - which is also why telegram:api reads fail here: the bot has never
// completed an API call, and that pre-poll state IS the correct answer.
//
// The test is deliberately SERIAL: t.Setenv mutates the process environment
// and [testing.T.Setenv] panics under [testing.T.Parallel]; paralleltest
// skips functions that contain it. Exactly one fx graph exists in this test
// binary, so fiberfx's process-wide prometheus registration stays unique.

import (
	"context"
	"testing"

	"github.com/capcom6/phone2tg-proxy/internal/bot"
	"github.com/capcom6/phone2tg-proxy/internal/config"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/proxy"
	"github.com/capcom6/phone2tg-proxy/internal/server"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/capcom6/phone2tg-proxy/internal/telegram"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/healthfx"
	"github.com/go-core-fx/httpfx"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/redisfx"
	"github.com/go-core-fx/telegofx"
	"github.com/go-core-fx/validatorfx"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// appGraphToken matches telego's ^\d+:[\w-]{35}$ format so telegofx.New
// passes the token gate at construction; no network call happens.
const appGraphToken = "123456:AAE-characterization-token-not-real"

// TestAppGraphServesTelegramLiveness builds the app.go graph from the env
// path and asserts the production wiring serves the telegram:api check on
// liveness - and only on liveness, matching the README's ready/startup note.
func TestAppGraphServesTelegramLiveness(t *testing.T) {
	t.Setenv("HTTP__ADDRESS", "127.0.0.1:0")
	t.Setenv("TELEGRAM__TOKEN", appGraphToken)
	t.Setenv("TELEGRAM__PROXY_URL", "")
	t.Setenv("STORAGE__SECRET", "app-graph-test-secret")
	// The suite runs with cwd internal/, one directory below the repo root
	// where the default i18n/locales lives.
	t.Setenv("I18N__TRANSLATIONS_PATH", "../i18n/locales")

	var healthSvc *healthfx.Service

	err := fx.New(
		logger.Module(),
		logger.WithFxDefaultLogger(),
		fiberfx.Module(),
		healthfx.Module(),
		// App-wide default HTTP client (httpfx defaults); the telegram module
		// overrides it with its own private client inside telegram.Module.
		httpfx.Module(),
		//
		config.Module(),
		telegram.Module(),
		telegofx.Module(telegofx.WithRouter(), telegofx.WithFSM(), telegofx.WithHealth()),
		redisfx.Module(),
		validatorfx.Module(),
		//
		fx.Supply(healthfx.Version{Version: "test"}),
		storage.Module(),
		server.Module(),
		bot.Module(),
		proxy.Module(),
		i18n.Module(),
		//
		fx.Populate(&healthSvc),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
	).Err()
	if err != nil {
		t.Fatalf("construct the production app graph: %v", err)
	}

	live := healthSvc.CheckLiveness(context.Background()).Checks

	detail, ok := live["telegram:api"]
	if !ok {
		t.Fatalf("liveness checks = %v, want the telegram:api key from telegofx.WithHealth", live)
	}
	if detail.Status != healthfx.StatusFail {
		t.Errorf("telegram:api status = %q before the bot has polled, want fail", detail.Status)
	}
	if _, hasSystem := live["system:goroutines"]; !hasSystem {
		t.Errorf("liveness checks = %v, want the system:goroutines key", live)
	}

	ready := healthSvc.CheckReadiness(context.Background()).Checks
	if _, inReady := ready["telegram:api"]; inReady {
		t.Errorf("readiness checks = %v, want no telegram:api: the provider serves liveness only", ready)
	}
}
