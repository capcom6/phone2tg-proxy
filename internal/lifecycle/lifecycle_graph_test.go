// Package lifecycle_test holds the fx Start -> Stop lifecycle coverage for the
// server slice.
//
// It is a SEPARATE test package - therefore a separate test binary, therefore a
// separate process - from internal/server, and that isolation is required for
// two independent reasons.
//
// First, two fiber apps cannot coexist in one process: server.Module() enables
// prometheus metrics and fiberfx/prometheus.Register hardcodes
// fiberprometheus.NewWithDefaultRegistry, which panics on a duplicate
// registration into the process-wide prometheus default registerer. fiberfx
// v0.6.0 has no registry seam to avoid it. One graph per process is a hard
// ceiling, so the graph that binds a listener and the graph that asserts routes
// cannot share a binary.
//
// Second, binding a listener and shutting it down reaches an acknowledged
// upstream data race in fasthttp; the two build-tag variants of this suite keep
// that race away from `make test`. The precise description lives on the test
// that owns each variant, lifecycle_test.go (!race) and lifecycle_race_test.go
// (race).
//
// Everything in this untagged file is shared by both variants, so the graph they
// build is identical: only the lifecycle they drive differs.
package lifecycle_test

import (
	"context"
	"errors"
	"testing"

	"github.com/capcom6/phone2tg-proxy/internal/proxy"
	"github.com/capcom6/phone2tg-proxy/internal/server"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/fiberfx/openapi"
	"github.com/go-core-fx/healthfx"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/validatorfx"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
)

// loopbackEphemeral asks the kernel for a free ephemeral loopback port. Binding
// :0 rather than a fixed port is what keeps the suite parallel-safe and free of
// collisions with anything already listening on the machine.
const loopbackEphemeral = "127.0.0.1:0"

// routeHealthLive is the liveness probe registered by healthfx. It is the
// cheapest end-to-end request: it needs no body, no headers and no validation.
const routeHealthLive = "/health/live"

// errStubSend is what the stub proxy always returns. The stub exists so the
// graph can boot without Redis and Telegram; no test sends a message, so the
// value only has to be a non-nil error.
var errStubSend = errors.New("stub proxy send")

// stubProxy satisfies proxy.Service without touching Redis or Telegram. It is
// supplied in place of proxy.Module(true), whose real dependencies are
// storage.Service and *telebot.Bot and would make the graph non-hermetic.
type stubProxy struct{}

// Send implements proxy.Service.
func (stubProxy) Send(_ context.Context, _, _ string) (int, error) {
	return 0, errStubSend
}

// newTestVersion mirrors the healthfx.Version main.go hands to internal.Run,
// including ReleaseID 0, which makes the omitempty releaseId field disappear.
func newTestVersion() healthfx.Version {
	return healthfx.Version{
		Version:   "dev",
		ReleaseID: 0,
		BuildDate: "",
		GitCommit: "",
		GoVersion: "",
	}
}

// newLifecycleGraph builds the production server graph from internal/app.go,
// minus every module that needs an external service, and returns the fx app and
// the fiber app.
//
// The graph is deliberately a full one rather than a stripped-down substitute:
// the point of this suite is that the wiring production ships starts, serves and
// stops. Three things are replaced, and only these three:
//
//   - fiberfx.Config.Address, so the listener is loopback-only and ephemeral;
//   - proxy.Service, replaced by stubProxy;
//   - the version, supplied here because main.go normally does it.
//
// openapi.Config mirrors what internal/config produces, so the docs routes are
// exercised as shipped.
//
// The app is returned CONSTRUCTED but NOT STARTED: fx runs every fx.Invoke while
// constructing, so all routes are registered by the time this returns, and each
// caller decides whether to drive the lifecycle.
func newLifecycleGraph(t *testing.T, address string) (*fx.App, *fiber.App) {
	t.Helper()

	var app *fiber.App

	fxApp := fx.New(
		// CORE MODULES.
		logger.Module(),
		logger.WithFxDefaultLogger(),

		fiberfx.Module(),
		fx.Supply(fiberfx.Config{
			Address:     address,
			ProxyHeader: "",
			Proxies:     []string{},
		}),

		healthfx.Module(),
		validatorfx.Module(),
		//
		// TEST-SUPPLIED VALUES.
		fx.Supply(newTestVersion()),
		fx.Supply(openapi.Config{
			Enabled:    true,
			PublicHost: "",
			PublicPath: "",
		}),
		fx.Provide(func() proxy.Service { return stubProxy{} }),
		//
		// APP MODULES.
		server.Module(),

		fx.Populate(&app),
	)

	// fx.New logs a construction failure instead of returning one, so read the
	// error back off the app. Without this the failure would surface later as a
	// confusing nil dereference in the caller's assertions.
	if err := fxApp.Err(); err != nil {
		t.Fatalf("construct the server fx graph: %v", err)
	}

	return fxApp, app
}
