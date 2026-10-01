package server_test

import (
	"context"
	"errors"
	"fmt"
	"os"
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

// errStubSend is returned by the stub proxy, which exists so the graph can boot
// without Redis or Telegram.
var errStubSend = errors.New("stub proxy send")

// sharedApp is the single fiber app this package's tests assert against. It is
// populated by TestMain while the fx graph is constructed.
var sharedApp *fiber.App

// stubProxy satisfies proxy.Service without touching Redis or Telegram.
type stubProxy struct{}

func (stubProxy) Send(_ context.Context, _, _ string) (int, error) {
	return 0, errStubSend
}

// newTestGraph builds the real server fx graph - the wiring internal/app.go and
// internal/server/module.go register in production - with the two external
// dependencies replaced: fiberfx.Config by an ephemeral 127.0.0.1:0 address and
// proxy.Service by stubProxy. openapi.Config mirrors the default internal/config
// produces, so the docs routes are exercised as shipped.
func newTestGraph() *fx.App {
	return fx.New(
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
		// Mirrors the internal/app.go provider verbatim, including ReleaseID 0.
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
		fx.Supply(openapi.Config{
			Enabled:    true,
			PublicHost: "",
			PublicPath: "",
		}),

		server.Module(),

		fx.Populate(&sharedApp),
	)
}

// TestMain owns the fx graph for this test package.
//
// Exactly one graph is built here, in the production wiring, and every test
// asserts against it through newWiredHealthApp. A second fiber app in the same
// process is impossible: server.Module() calls opts.WithMetrics(), and
// fiberfx/prometheus.Register hardcodes fiberprometheus.NewWithDefaultRegistry,
// which panics on a duplicate registration into the process-wide prometheus
// default registerer. fiberfx v0.6.0 has no registry seam to avoid it.
//
// The graph is built but deliberately NOT started. fx runs every fx.Invoke while
// constructing, so every route is already registered once fx.New returns, and
// app.Test serves them without a listener - which is all these tests need.
//
// Starting it would instead race: fasthttp.Server.Serve sets Server.done to a
// non-nil channel (server.go:1970), so RequestCtx.Done stops returning nil, so
// the healthfx probe's context.WithTimeout(c.Context(), 5s) spawns a
// propagateCancel goroutine that reads RequestCtx.s.done - the exact field
// fasthttp.Server.ShutdownWithContext writes to nil (server.go:2085) while
// holding no lock it shares with that reader. That is an acknowledged upstream
// data race, it fired in 2 of 3 -race runs here, and nothing in this repository
// can add the missing synchronization edge. The address 127.0.0.1:0 is kept so
// the wiring stays identical to a bound listener's, minus the listener.
//
// The lifecycle coverage that used to live in this TestMain now lives in
// internal/lifecycle, as its own test package with its own test binary: one
// graph per process is a hard ceiling here, so a graph that binds a listener
// cannot share this binary. That package binds a real loopback listener and
// asserts Start, Stop and the port release in a non-race build; see the build
// tag on internal/lifecycle/lifecycle_test.go for the exact reasoning.
//
// fx.New logs a construction failure instead of returning one, so the error is
// surfaced here: the process exits non-zero with one clear line instead of every
// test failing with a misleading "fiber app was not populated".
func TestMain(m *testing.M) {
	newTestGraph()

	if sharedApp == nil {
		fmt.Fprintln(os.Stderr, "server_test: fx graph construction failed, see the fx log above")
		os.Exit(1)
	}

	os.Exit(m.Run())
}
