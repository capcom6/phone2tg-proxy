package server_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/capcom6/phone2tg-proxy/internal/proxy"
	"github.com/capcom6/phone2tg-proxy/internal/server"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/fiberfx/health"
	"github.com/go-core-fx/healthfx"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/validatorfx"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
	"go.uber.org/zap/zaptest"
)

// healthTestTimeoutMS is the app.Test timeout in milliseconds.
const healthTestTimeoutMS = 5000

// The four health routes registered by the T101 wiring, in a stable order.
const (
	routeHealth        = "/health"
	routeHealthLive    = "/health/live"
	routeHealthReady   = "/health/ready"
	routeHealthStartup = "/health/startup"
)

// Test-fixture sentinels. They are literals invented for this test file - no
// real secret is ever read from the gitignored .env or hardcoded here.
const (
	sentinelToken  = "111111:SENTINEL-telegram-token"
	sentinelSecret = "SENTINEL-storage-secret"
	sentinelPhone  = "+15550100"
)

// healthRoutePaths returns the full route matrix under test.
func healthRoutePaths() []string {
	return []string{routeHealth, routeHealthLive, routeHealthReady, routeHealthStartup}
}

// newWiredHealthApp builds the real server fx graph - the exact wiring
// internal/app.go and internal/server/module.go register in production - and
// returns the app with its routes already registered.
//
// The graph is deliberately NOT started. fx runs every fx.Invoke while
// constructing the graph, so the health routes are registered by the time fx.New
// returns, and app.Test serves them without a bound listener. That is all these
// tests need: they assert route registration and response contracts, neither of
// which requires a listening socket. The Start -> Stop lifecycle over a bound
// listener stays covered by TestAppBootSmoke.
func newWiredHealthApp(t *testing.T) *fiber.App {
	t.Helper()

	var app *fiber.App

	fx.New(
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

		server.Module(),

		fx.Populate(&app),
	)

	if app == nil {
		t.Fatal("fiber app was not populated")
	}

	return app
}

// getHealth performs a GET against the fiber app and returns the status code
// plus the raw, undecoded response body.
func getHealth(t *testing.T, app *fiber.App, path string) (int, string) {
	t.Helper()

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil), healthTestTimeoutMS)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body of GET %s: %v", path, err)
	}

	return resp.StatusCode, string(body)
}

// TestHealthRouteMatrix measures the status code of every health route and pins
// the writeProbe contract: 200 while the overall status is not "fail", 503 once
// it is (fiberfx/health health.go:77-83).
func TestHealthRouteMatrix(t *testing.T) {
	app := newWiredHealthApp(t)

	for _, path := range healthRoutePaths() {
		code, body := getHealth(t, app, path)

		if code != http.StatusOK && code != http.StatusServiceUnavailable {
			t.Errorf("GET %s: status = %d, want 200 or 503 (body: %s)", path, code, body)
		}

		t.Logf("GET %s -> %d %s", path, code, body)
	}
}

// stubHealthProvider is a deterministic healthfx.Provider. Every probe returns
// the same fixed check detail, so the 200/503 contract can be measured without
// depending on live runtime metrics such as goroutine count.
type stubHealthProvider struct {
	name   string
	status healthfx.Status
}

func (p stubHealthProvider) Name() string {
	return p.name
}

func (p stubHealthProvider) StartedProbe(_ context.Context) (healthfx.Checks, error) {
	return p.checks(), nil
}

func (p stubHealthProvider) ReadyProbe(_ context.Context) (healthfx.Checks, error) {
	return p.checks(), nil
}

func (p stubHealthProvider) LiveProbe(_ context.Context) (healthfx.Checks, error) {
	return p.checks(), nil
}

func (p stubHealthProvider) checks() healthfx.Checks {
	return healthfx.Checks{
		"probe": healthfx.CheckDetail{
			Description:   "deterministic stub probe",
			ObservedUnit:  "unit",
			ObservedValue: 1,
			Status:        p.status,
		},
	}
}

// newStubHealthApp registers the production health handler over a deterministic
// provider set, so each route can be driven into a known overall status.
func newStubHealthApp(t *testing.T, providers ...healthfx.Provider) *fiber.App {
	t.Helper()

	svc := healthfx.NewService(
		providers,
		healthfx.Version{
			Version:   "dev",
			ReleaseID: 0,
			BuildDate: "",
			GitCommit: "",
			GoVersion: "",
		},
		zaptest.NewLogger(t),
	)

	app := fiber.New()
	health.NewHandler(svc).Register(app)

	return app
}

// TestHealthProbeStatusMatrix drives all four routes through every overall
// status and asserts the measured HTTP status code. Only "fail" maps to 503;
// "warn" - which the built-in system provider uses above 100 goroutines or
// 128MiB of heap - still maps to 200 (F-10).
func TestHealthProbeStatusMatrix(t *testing.T) {
	cases := []struct {
		name   string
		status healthfx.Status
		want   int
	}{
		{name: "pass maps to 200", status: healthfx.StatusPass, want: http.StatusOK},
		{name: "warn maps to 200", status: healthfx.StatusWarn, want: http.StatusOK},
		{name: "fail maps to 503", status: healthfx.StatusFail, want: http.StatusServiceUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newStubHealthApp(t, stubHealthProvider{name: "stub", status: tc.status})

			for _, path := range healthRoutePaths() {
				code, body := getHealth(t, app, path)
				if code != tc.want {
					t.Errorf("GET %s with overall status %q: status = %d, want %d (body: %s)",
						path, tc.status, code, tc.want, body)
				}
			}
		})
	}
}

// topLevelAllowlist is the exhaustive set of top-level keys a health response
// may carry (fiberfx/health dto.go:47-58).
func topLevelAllowlist() map[string]struct{} {
	return map[string]struct{}{
		"status":    {},
		"version":   {},
		"releaseId": {},
		"checks":    {},
	}
}

// checkAllowlist is the exhaustive set of keys a single checks[name] object may
// carry (fiberfx/health dto.go:21-31).
func checkAllowlist() map[string]struct{} {
	return map[string]struct{}{
		"description":   {},
		"observedUnit":  {},
		"observedValue": {},
		"status":        {},
	}
}

// assertTopLevelWhitelist asserts the top-level keys of a decoded health body.
// status is always required; every other key must be whitelisted; and releaseId
// must be absent because the app reports ReleaseID 0 and the field carries
// omitempty (dto.go:55). Assert the absence; do not "fix" the dto.
func assertTopLevelWhitelist(t *testing.T, path string, top map[string]json.RawMessage) {
	t.Helper()

	for _, key := range slices.Sorted(maps.Keys(top)) {
		if _, ok := topLevelAllowlist()[key]; !ok {
			t.Errorf("GET %s: top-level key %q is not whitelisted", path, key)
		}
	}

	if _, ok := top["status"]; !ok {
		t.Errorf("GET %s: top-level key %q must always be present", path, "status")
	}

	if raw, ok := top["releaseId"]; ok {
		t.Errorf("GET %s: key %q must be absent at ReleaseID 0, got %s", path, "releaseId", raw)
	}
}

// assertChecksWhitelist asserts every key of every checks[name] object is
// whitelisted, and that the built-in system provider reports exactly the two
// expected checks (F-10).
func assertChecksWhitelist(t *testing.T, path, raw string) {
	t.Helper()

	var checks map[string]map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &checks); err != nil {
		t.Fatalf("GET %s: decode checks %s: %v", path, raw, err)
	}

	for _, name := range slices.Sorted(maps.Keys(checks)) {
		for _, key := range slices.Sorted(maps.Keys(checks[name])) {
			if _, allowed := checkAllowlist()[key]; !allowed {
				t.Errorf("GET %s: checks[%q] key %q is not whitelisted", path, name, key)
			}
		}
	}

	want := []string{"system:goroutines", "system:memory"}
	if got := slices.Sorted(maps.Keys(checks)); !slices.Equal(got, want) {
		t.Errorf("GET %s: checks = %v, want %v", path, got, want)
	}
}

// TestHealthBodyKeyWhitelist is a whitelist, not a blocklist: it proves no key
// outside the dto can appear, so no secret can hide in an unanticipated field.
func TestHealthBodyKeyWhitelist(t *testing.T) {
	app := newWiredHealthApp(t)

	for _, path := range healthRoutePaths() {
		code, body := getHealth(t, app, path)
		if code != http.StatusOK && code != http.StatusServiceUnavailable {
			t.Fatalf("GET %s: status = %d, want 200 or 503", path, code)
		}

		var top map[string]json.RawMessage
		if err := json.Unmarshal([]byte(body), &top); err != nil {
			t.Fatalf("GET %s: decode body %q: %v", path, body, err)
		}

		assertTopLevelWhitelist(t, path, top)

		rawChecks, hasChecks := top["checks"]

		// No provider registers a readiness or startup check, so healthfx returns an
		// empty map and omitempty drops the key (dto.go:57). Pin that absence.
		if path == routeHealthReady || path == routeHealthStartup {
			if hasChecks {
				t.Errorf("GET %s: key %q must be absent while no provider reports checks, got %s",
					path, "checks", rawChecks)
			}

			continue
		}

		if !hasChecks {
			t.Errorf("GET %s: top-level key %q must be present", path, "checks")

			continue
		}

		assertChecksWhitelist(t, path, string(rawChecks))
	}
}

// digitsOnly mirrors internal/storage.normalizePhoneNumber: every non-digit is
// dropped, which is the exact input the production HMAC is computed over.
func digitsOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}

		return -1
	}, s)
}

// sentinelPhoneHMAC mirrors internal/storage.hmacPhone: lowercase hex of
// HMAC-SHA256 keyed with the storage secret over the digit-only phone number.
func sentinelPhoneHMAC(secret, phone string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(digitsOnly(phone)))

	return hex.EncodeToString(mac.Sum(nil))
}

// TestHealthBodiesLeakNoSentinels is the negative half of the whitelist: four
// literal sentinels - the test token, the test secret, a planted phone number,
// and the HMAC hex of that phone computed with that secret - must be absent from
// the raw body of every route. It is a whitelist test's complement, not a
// replacement: a leak through a new key would be caught by the whitelist test
// above, whereas this one only proves these four known values do not leak.
//
// The superseded "no 10+ digit run" regex assertion stays deleted: a digit run
// proves nothing about which digits were emitted.
func TestHealthBodiesLeakNoSentinels(t *testing.T) {
	// Plant the sentinels in the process environment exactly as koanf would read
	// them, so the health slice is exercised with secrets present.
	t.Setenv("TELEGRAM__TOKEN", sentinelToken)
	t.Setenv("STORAGE__SECRET", sentinelSecret)

	phoneHMAC := sentinelPhoneHMAC(sentinelSecret, sentinelPhone)

	sentinels := []string{
		sentinelToken,
		sentinelSecret,
		sentinelPhone,
		phoneHMAC,
	}

	app := newWiredHealthApp(t)

	for _, path := range healthRoutePaths() {
		_, body := getHealth(t, app, path)

		for _, sentinel := range sentinels {
			if strings.Contains(body, sentinel) {
				t.Errorf("GET %s: response body leaks a planted sentinel %q: %s", path, sentinel, body)
			}
		}
	}
}

// routeMessages is the only public API route, registered by
// internal/server/module.go.
const routeMessages = "/api/v1/messages"

// postJSON posts body verbatim as application/json and returns the status code
// plus the raw, undecoded response body.
func postJSON(t *testing.T, app *fiber.App, path, body string) (int, string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := app.Test(req, healthTestTimeoutMS)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}

	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body of POST %s: %v", path, err)
	}

	return resp.StatusCode, string(raw)
}

// errorKeyAllowlist is the exhaustive set of top-level keys a JSON error
// response may carry (pkg/client/responses.go:3-10).
func errorKeyAllowlist() map[string]struct{} {
	return map[string]struct{}{
		"message": {},
		"code":    {},
		"details": {},
	}
}

// plantedSentinels returns the four planted values that must never appear in a
// response body: the test token, the test secret, a phone number, and the HMAC
// hex of that phone keyed with that secret.
func plantedSentinels() []string {
	return []string{
		sentinelToken,
		sentinelSecret,
		sentinelPhone,
		sentinelPhoneHMAC(sentinelSecret, sentinelPhone),
	}
}

// assertNoSentinel fails if body carries any planted sentinel.
func assertNoSentinel(t *testing.T, path, body string) {
	t.Helper()

	for _, sentinel := range plantedSentinels() {
		if strings.Contains(body, sentinel) {
			t.Errorf("POST %s: response body leaks a planted sentinel %q: %s", path, sentinel, body)
		}
	}
}

// decodeRequired decodes a required envelope key into out and reports whether
// the key was both present and decodable.
func decodeRequired(t *testing.T, path string, envelope map[string]json.RawMessage, key string, out any) bool {
	t.Helper()

	raw, ok := envelope[key]
	if !ok {
		t.Errorf("POST %s: key %q must be present", path, key)

		return false
	}

	if err := json.Unmarshal(raw, out); err != nil {
		t.Errorf("POST %s: decode key %q value %s: %v", path, key, raw, err)

		return false
	}

	return true
}

// assertErrorEnvelope asserts the standard JSON error contract: only whitelisted
// top-level keys, a non-empty message, and code equal to wantCode. It does NOT
// assert the validator-generated wording inside message, which is owned by
// go-playground/validator and not by this service.
func assertErrorEnvelope(t *testing.T, path, body string, wantCode int) {
	t.Helper()

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("POST %s: decode body %q: %v", path, body, err)
	}

	for key := range envelope {
		if _, allowed := errorKeyAllowlist()[key]; !allowed {
			t.Errorf("POST %s: key %q is not whitelisted", path, key)
		}
	}

	var message string
	if !decodeRequired(t, path, envelope, "message", &message) {
		return
	}

	if strings.TrimSpace(message) == "" {
		t.Errorf("POST %s: key %q must be non-empty, got %q", path, "message", message)
	}

	var code int
	if !decodeRequired(t, path, envelope, "code", &code) {
		return
	}

	if code != wantCode {
		t.Errorf("POST %s: key %q = %d, want %d", path, "code", code, wantCode)
	}
}

// TestMessagesRouteCoexistsWithHealth is the automated assertion for the wave's
// route coexistence criterion: registering the four health routes must not
// disturb the only public API surface. An empty JSON body is still rejected
// with 400 and the standard {"message", "code"} envelope. Every health test
// would stay green if this broke, which is exactly why it needs its own test.
func TestMessagesRouteCoexistsWithHealth(t *testing.T) {
	// Plant the sentinels in the process environment exactly as koanf would read
	// them, so coexistence is measured with secrets present.
	t.Setenv("TELEGRAM__TOKEN", sentinelToken)
	t.Setenv("STORAGE__SECRET", sentinelSecret)

	app := newWiredHealthApp(t)

	code, body := postJSON(t, app, routeMessages, "{}")
	if code != http.StatusBadRequest {
		t.Fatalf("POST %s with body {}: status = %d, want 400 (body: %s)", routeMessages, code, body)
	}

	assertErrorEnvelope(t, routeMessages, body, http.StatusBadRequest)
	assertNoSentinel(t, routeMessages, body)
}
