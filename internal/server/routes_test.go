package server_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// routeKey renders one registered route as "METHOD path" so a whole route table
// can be compared as one sorted string set.
func routeKey(method, path string) string {
	return method + " " + path
}

// handlerRoutes returns the sorted "METHOD path" keys of the routes a request can
// actually be dispatched to.
//
// GetRoutes(true) filters out the Use registrations. That filter is what makes
// this table meaningful: app.Group and app.Use register the bare prefix once per
// HTTP verb, so an unfiltered listing always contains entries such as
// "POST /api/v1" even when nothing is bound to them. Only non-Use routes are
// evidence that a handler really registered.
func handlerRoutes(t *testing.T) []string {
	t.Helper()

	app := newWiredHealthApp(t)

	routes := app.GetRoutes(true)
	got := make([]string, 0, len(routes))
	for _, route := range routes {
		got = append(got, routeKey(route.Method, route.Path))
	}

	slices.Sort(got)

	return got
}

// TestRegisteredRouteTable pins the complete dispatchable route table.
//
// This is the assertion whose absence let two silent defects through. A dig
// value group is keyed by the EXACT result type, so a constructor returning
// *MessagesHandler resolved to an EMPTY []handler.Handler slice - no error, no
// warning, the route simply never registered. Separately, a handler that stops
// creating its own sub-group silently relocates its route from
// /api/v1/messages to /api/v1. Neither fault breaks compilation and neither
// fails a graph construction; both only ever show up as a 404 in production.
// Pinning the exact table turns both into loud, local failures.
func TestRegisteredRouteTable(t *testing.T) {
	want := []string{
		routeKey(http.MethodGet, routeHealth),
		routeKey(http.MethodGet, routeHealthLive),
		routeKey(http.MethodGet, routeHealthReady),
		routeKey(http.MethodGet, routeHealthStartup),
		routeKey(http.MethodGet, "/metrics"),
		// Fiber answers every GET route on HEAD as well.
		routeKey(http.MethodHead, routeHealth),
		routeKey(http.MethodHead, routeHealthLive),
		routeKey(http.MethodHead, routeHealthReady),
		routeKey(http.MethodHead, routeHealthStartup),
		routeKey(http.MethodHead, "/metrics"),
		routeKey(http.MethodPost, routeMessages),
	}

	if got := handlerRoutes(t); !slices.Equal(got, want) {
		t.Errorf("dispatchable routes = %v,\nwant %v", got, want)
	}
}

// TestMessagesRouteIsRegisteredAtThePublicPath asserts the messages handler is
// bound to /api/v1/messages specifically, and that it did NOT drift onto the
// bare v1 group prefix.
//
// This has to assert on the route table rather than on a response status: both
// paths answer. One answers with the handler, the other with fiber's default 404,
// and a 404 is exactly what the defect produced.
func TestMessagesRouteIsRegisteredAtThePublicPath(t *testing.T) {
	wantKey := routeKey(http.MethodPost, routeMessages)
	driftKey := routeKey(http.MethodPost, "/api/v1")

	routes := handlerRoutes(t)

	if !slices.Contains(routes, wantKey) {
		t.Errorf("%q is not registered, routes = %v", wantKey, routes)
	}

	if slices.Contains(routes, driftKey) {
		t.Errorf("%q is registered; the handler lost its /messages sub-group", driftKey)
	}
}

// TestOpenAPIDocsAreRegistered asserts the OpenAPI mount exists. The docs are
// served by a single wildcard Use mount rather than by per-file routes, so this
// one has to read the unfiltered route list.
func TestOpenAPIDocsAreRegistered(t *testing.T) {
	app := newWiredHealthApp(t)

	want := routeKey(http.MethodGet, "/api/v1/docs/*")

	var found bool

	for _, route := range app.GetRoutes() {
		if routeKey(route.Method, route.Path) == want {
			found = true
		}
	}

	if !found {
		t.Errorf("%q is not registered", want)
	}
}

// TestOpenAPIDocJSONServesThisServiceSpec asserts the served spec is this
// service's own, not a foreign one.
//
// The docs handler reads whichever *swag.Spec the graph supplied. Supplying the
// wrong docs package compiles cleanly and yields a perfectly well-formed spec for
// entirely different routes, so the title is the cheapest proof that the right
// spec reached the handler.
func TestOpenAPIDocJSONServesThisServiceSpec(t *testing.T) {
	app := newWiredHealthApp(t)

	code, body := getHealth(t, app, "/api/v1/docs/doc.json")
	if code != http.StatusOK {
		t.Fatalf("GET /api/v1/docs/doc.json: status = %d, want 200 (body: %s)", code, body)
	}

	var spec struct {
		Info struct {
			Title string `json:"title"`
		} `json:"info"`
		BasePath string `json:"basePath"`
	}

	if err := json.Unmarshal([]byte(body), &spec); err != nil {
		t.Fatalf("GET /api/v1/docs/doc.json: decode spec: %v (body: %s)", err, body)
	}

	if spec.Info.Title != "Phone Number to Telegram Proxy" {
		t.Errorf("spec title = %q, want %q - the graph served a foreign spec",
			spec.Info.Title, "Phone Number to Telegram Proxy")
	}

	if spec.BasePath != routeAPIBase {
		t.Errorf("spec basePath = %q, want %q", spec.BasePath, routeAPIBase)
	}
}

// TestMessagesEmptyBodyRejectionShape pins the exact rejection contract of the
// messages route: 400 with a non-empty message that keeps the handler's own
// "invalid request: validation failed: " prefix, and with no details key.
//
// The absent details key is asserted, not merely allowed: fiberfx's JSON error
// response is built with a nil details value, and the response carries
// omitempty, so the key must not appear.
func TestMessagesEmptyBodyRejectionShape(t *testing.T) {
	t.Setenv("TELEGRAM__TOKEN", sentinelToken)
	t.Setenv("STORAGE__SECRET", sentinelSecret)

	app := newWiredHealthApp(t)

	code, body := postJSON(t, app, routeMessages, "{}")
	if code != http.StatusBadRequest {
		t.Fatalf("POST %s with body {}: status = %d, want 400 (body: %s)", routeMessages, code, body)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("POST %s: decode body %q: %v", routeMessages, body, err)
	}

	if _, ok := envelope["details"]; ok {
		t.Errorf("POST %s: key %q must be absent, got %s", routeMessages, "details", envelope["details"])
	}

	var message string
	if raw, ok := envelope["message"]; !ok || json.Unmarshal(raw, &message) != nil {
		t.Fatalf("POST %s: key %q must be present and decodable, got %s", routeMessages, "message", raw)
	}

	if !strings.HasPrefix(message, "invalid request: validation failed: ") {
		t.Errorf("POST %s: message = %q, want prefix %q",
			routeMessages, message, "invalid request: validation failed: ")
	}
}

// TestBareAPIPrefixIsNotAMessagesHandler is the behavioural half of
// TestMessagesRouteIsRegisteredAtThePublicPath: it proves the drift path really
// is unrouted, so a passing table assertion cannot be an artifact of a stray
// catch-all.
func TestBareAPIPrefixIsNotAMessagesHandler(t *testing.T) {
	app := newWiredHealthApp(t)

	code, body := postJSON(t, app, routeAPIBase, "{}")
	if code != http.StatusNotFound {
		t.Errorf("POST %s: status = %d, want 404 (body: %s)", routeAPIBase, code, body)
	}
}
