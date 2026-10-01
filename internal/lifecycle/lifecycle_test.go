//go:build !race

// TestStartStopLifecycleBindsAndReleasesTheListener owns the real Start -> Stop
// lifecycle, and it is compiled ONLY when -race is off.
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
// graceful shutdown, which is the only thing this file does.
//
// Nothing in this repository can add the missing synchronization edge, and
// neither can the test harness: the two writes are both inside fasthttp. So the
// lifecycle assertions are compiled out of -race builds and kept here, where
// they run for every non-race `go test` / `go build` invocation. The -race
// counterpart in lifecycle_race_test.go keeps the graph construction and route
// registration assertions in the race build so this package is never silently
// skipped.
//
// Anything this file stops asserting as a result is the lifecycle only: the
// route table, the health body shape and the sentinel-leak checks all stay in
// internal/server, where they run under -race today.

package lifecycle_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// Bounds for the lifecycle calls and for the listener probes around them. They
// are generous on purpose: a slow machine should not turn a shutdown that is
// merely late into a failure.
const (
	// startTimeout bounds fxApp.Start, which includes binding the listener.
	startTimeout = 15 * time.Second

	// stopTimeout bounds fxApp.Stop, which includes the graceful drain.
	stopTimeout = 15 * time.Second

	// requestTimeout bounds one probe issued against the bound listener.
	requestTimeout = 5 * time.Second

	// dialTimeout bounds one TCP dial attempt.
	dialTimeout = time.Second

	// listenerReleaseTimeout bounds how long Stop may take to give the port
	// back before the port is reported as still held.
	listenerReleaseTimeout = 5 * time.Second

	// listenerReleasePollInterval is the gap between release probes.
	listenerReleasePollInterval = 20 * time.Millisecond
)

// reserveLocalAddress binds an ephemeral loopback listener and returns it
// together with its address.
//
// The reservation is held while the fx graph is built and released immediately
// before Start, so the gap during which another process could steal the port is
// only the distance between the two calls. fiberfx then binds the address itself,
// in its own OnStart hook, which is exactly what production does with a
// configured address.
func reserveLocalAddress(t *testing.T) (net.Listener, string) {
	t.Helper()

	ln, err := net.Listen("tcp", loopbackEphemeral)
	if err != nil {
		t.Fatalf("reserve an ephemeral loopback address: %v", err)
	}

	return ln, ln.Addr().String()
}

// releaseReservation hands the reserved port back to the kernel so fiberfx's
// OnStart hook can bind it.
func releaseReservation(t *testing.T, ln net.Listener) {
	t.Helper()

	if err := ln.Close(); err != nil {
		t.Fatalf("release the reserved address: %v", err)
	}
}

// probeLiveHealth issues one real GET at the bound listener and returns the
// status code plus the raw body.
//
// It deliberately does not use fiber's app.Test: the point is to go through the
// kernel TCP stack, so a 200 proves a listener is bound and serving rather than
// merely that a route exists.
func probeLiveHealth(t *testing.T, address string) (int, string) {
	t.Helper()

	url := "http://" + address + routeHealthLive

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build a GET request for %s: %v", url, err)
	}

	client := &http.Client{Timeout: requestTimeout}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s through the bound listener: %v", url, err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read the body of GET %s: %v", url, err)
	}

	return resp.StatusCode, string(body)
}

// assertListenerReleased asserts that nothing accepts connections on address
// any more. It retries until the deadline, because the listener close is
// observed slightly after Stop returns.
func assertListenerReleased(t *testing.T, address string) {
	t.Helper()

	deadline := time.Now().Add(listenerReleaseTimeout)

	for {
		conn, err := net.DialTimeout("tcp", address, dialTimeout)
		if err != nil {
			return
		}

		err = conn.Close()
		if err != nil {
			t.Fatalf("close the release probe connection to %s: %v", address, err)
		}

		if time.Now().After(deadline) {
			t.Fatalf("listener %s still accepts connections %s after Stop", address, listenerReleaseTimeout)
		}

		time.Sleep(listenerReleasePollInterval)
	}
}

// TestStartStopLifecycleBindsAndReleasesTheListener drives the restored
// lifecycle coverage end to end:
//
//  1. the graph constructs with a reserved ephemeral loopback address;
//  2. Start binds it and the app serves a real request over it;
//  3. Stop returns without error;
//  4. the port is released, so the socket really was shut down.
//
// Assertions 2 and 4 are what make this more than a smoke test: the graph being
// constructible is already covered in internal/server, but no other test proves
// the OnStart listener hook binds, serves and gives the port back.
func TestStartStopLifecycleBindsAndReleasesTheListener(t *testing.T) {
	reservation, address := reserveLocalAddress(t)

	fxApp, _ := newLifecycleGraph(t, address)

	// Construction is done; hand the port over now, as late as possible.
	releaseReservation(t, reservation)

	var stopped bool

	startCtx, cancelStart := context.WithTimeout(t.Context(), startTimeout)
	defer cancelStart()

	if err := fxApp.Start(startCtx); err != nil {
		t.Fatalf("fxApp.Start: %v", err)
	}

	t.Cleanup(func() {
		if stopped {
			return
		}

		// t.Context() is already cancelled while cleanup runs, so the fallback
		// needs a context of its own.
		ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
		defer cancel()

		if err := fxApp.Stop(ctx); err != nil {
			t.Errorf("fxApp.Stop in cleanup: %v", err)
		}
	})

	code, body := probeLiveHealth(t, address)
	if code != http.StatusOK {
		t.Fatalf("GET %s through the bound listener: status = %d, want 200 (body: %s)", routeHealthLive, code, body)
	}

	t.Logf("bound listener %s served GET %s -> %d %s", address, routeHealthLive, code, body)

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("decode the health body %q: %v", body, err)
	}

	if _, ok := decoded["status"]; !ok {
		t.Errorf("GET %s: body %q has no %q key", routeHealthLive, body, "status")
	}

	stopCtx, cancelStop := context.WithTimeout(t.Context(), stopTimeout)
	defer cancelStop()

	if err := fxApp.Stop(stopCtx); err != nil {
		t.Fatalf("fxApp.Stop: %v", err)
	}

	stopped = true

	assertListenerReleased(t, address)

	t.Logf("Stop released listener %s; no connection is accepted on it any more", address)
}
