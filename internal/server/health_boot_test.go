package server_test

import (
	"net/http"
	"testing"
)

// TestAppBootSmoke asserts the four health routes are registered on the shared
// app.
//
// What this owns is route registration on a fully constructed graph, not the
// Start -> Stop lifecycle: TestMain builds and owns the graph, and the listener
// is never bound. The lifecycle is covered by
// TestStartStopLifecycleBindsAndReleasesTheListener in internal/lifecycle, a
// separate test package with its own test binary. See TestMain here for why
// binding the listener here would race under -race.
func TestAppBootSmoke(t *testing.T) {
	app := newWiredHealthApp(t)

	want := map[string]bool{
		routeHealth:        false,
		routeHealthLive:    false,
		routeHealthReady:   false,
		routeHealthStartup: false,
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
