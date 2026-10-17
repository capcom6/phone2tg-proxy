//go:build race

// This file is the -race half of the lifecycle suite, and it exists so that
// `make test` does not silently skip this package.
//
// The race it avoids is genuine and upstream, not a harness artifact. fasthttp
// v1.72.0 writes Server.done without synchronization that its reader honours:
//
//   - server.go:2082-2086 - the graceful shutdown loop sets `s.done = nil` once
//     open == 0. fasthttp's own comment right below it admits the shape is
//     unsound: "This is not an optimal solution but using a sync.WaitGroup here
//     causes data races".
//   - server.go:2992-2995 - func (ctx *RequestCtx) Done returns `ctx.s.done`
//     with no lock at all.
//
// Once the server is serving, Done stops returning nil, so any context derived
// from the fiber request context spawns a propagateCancel goroutine that reads
// that field. go-core-fx/healthfx v0.1.0 service.go:80 derives exactly such a
// context - probeCtx, cancel := context.WithTimeout(ctx, providerTimeout) - so a
// health request in flight while the listener shuts down can race the nil
// write. The request path itself does NOT race; the race is reachable from
// graceful shutdown only, which is why this file never binds a listener and
// never calls Start or Stop. The lifecycle assertions themselves live in
// lifecycle_test.go, compiled only when -race is off.
//
// Nothing in this repository can add the missing synchronization edge, and
// neither can the test harness: both writes are inside fasthttp.

package lifecycle_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// routeKey renders one registered route as "METHOD path" so a whole route table
// can be compared as one sorted string set.
func routeKey(method, path string) string {
	return method + " " + path
}

// dispatchableRoutes returns the sorted "METHOD path" keys of the routes a
// request can actually be dispatched to. GetRoutes(true) filters out the Use
// registrations, which otherwise make the bare /api/v1 prefix look routable.
func dispatchableRoutes(app *fiber.App) []string {
	routes := app.GetRoutes(true)

	got := make([]string, 0, len(routes))
	for _, route := range routes {
		got = append(got, routeKey(route.Method, route.Path))
	}

	slices.Sort(got)

	return got
}

// wantDispatchableRoutes is the route table the graph must register in the race
// build as well. It is the same table internal/server pins, because the graph is
// the same graph.
func wantDispatchableRoutes() []string {
	return []string{
		routeKey(http.MethodGet, "/health"),
		routeKey(http.MethodGet, "/health/live"),
		routeKey(http.MethodGet, "/health/ready"),
		routeKey(http.MethodGet, "/health/startup"),
		routeKey(http.MethodGet, "/metrics"),
		// Fiber answers every GET route on HEAD as well.
		routeKey(http.MethodHead, "/health"),
		routeKey(http.MethodHead, "/health/live"),
		routeKey(http.MethodHead, "/health/ready"),
		routeKey(http.MethodHead, "/health/startup"),
		routeKey(http.MethodHead, "/metrics"),
		routeKey(http.MethodPost, "/api/v1/messages"),
	}
}

// appTestLiveHealth drives the liveness route through fiber's in-process test
// transport. No listener is involved, so Serve is never entered and
// fasthttp.Server.done stays nil for the whole request - which is what makes
// this safe to run under -race.
func appTestLiveHealth(t *testing.T, app *fiber.App) (int, string) {
	t.Helper()

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, routeHealthLive, nil), 5000)
	if err != nil {
		t.Fatalf("GET %s: %v", routeHealthLive, err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read the body of GET %s: %v", routeHealthLive, err)
	}

	return resp.StatusCode, string(body)
}

// TestGraphConstructsAndServesUnderRace asserts everything a -race build can
// safely assert about this graph: it constructs, every route is registered, and
// a health request is answered. The listener is never bound, so Start and Stop
// are never called and the fasthttp shutdown race is unreachable.
//
// This is the assertion that keeps the package alive under `make test`: without
// it, `go test -race ./...` would report a package with no tests instead of
// silently passing.
func TestGraphConstructsAndServesUnderRace(t *testing.T) {
	// The address is never bound. It is still a real loopback address so the
	// wiring is byte-identical to the non-race variant.
	_, app := newLifecycleGraph(t, loopbackEphemeral)

	if got := dispatchableRoutes(app); !slices.Equal(got, wantDispatchableRoutes()) {
		t.Errorf("dispatchable routes = %v,\nwant %v", got, wantDispatchableRoutes())
	}

	code, body := appTestLiveHealth(t, app)
	if code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200 (body: %s)", routeHealthLive, code, body)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("decode the health body %q: %v", body, err)
	}

	if _, ok := decoded["status"]; !ok {
		t.Errorf("GET %s: body %q has no %q key", routeHealthLive, body, "status")
	}
}
