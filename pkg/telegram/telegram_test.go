// Package telegram_test drives pkg/telegram through the production fx wiring
// instead of calling its constructors directly.
//
// Every case builds the real module shape: logger.Module(), httpfx.Module(), an
// httpfx.Config provider, a telegram.Config provider and telegram.Module(). That
// is deliberate. The defect this suite exists to catch - "cannot provide
// *http.Client from [0]: already provided by httpfx.Module.func1" - is a
// WIRING defect: go build, go vet and golangci-lint were all green on the
// commit that carried it and only the running binary failed, which no suite
// that calls telegram.New directly can see.
//
// GRAPH COUNT. fx imposes no one-graph-per-process limit of its own, and this
// package MEASURES that building and discarding many graphs inside one test
// binary is safe: nothing in the logger, httpfx or telegram graph registers
// with the process-wide prometheus.DefaultRegisterer. That registration is what
// forces internal/server down to a single shared graph in TestMain - fiberfx
// hardcodes fiberprometheus.NewWithDefaultRegistry and panics on a second app.
// Here what varies is configuration, not wiring shape, so one graph per case is
// the thing under test.
//
// No graph is started. Start would run telegram.Module's OnStart hook, which
// launches bot.Start() and its long poll; every constructible failure in this
// module already surfaces from fx.New, so construction is the whole assertion.
package telegram_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/capcom6/phone2tg-proxy/pkg/telegram"
	"github.com/go-core-fx/httpfx"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"gopkg.in/telebot.v4"
)

// createBotPrefix is the operation prefix telegram.New wraps telebot failures with.
const createBotPrefix = "create bot: "

// failedClientBuild is the fx prefix dig puts in front of a constructor failure,
// so a rejected proxy URL is attributable to the client provider.
const failedClientBuild = "failed to build *http.Client"

// validToken is well formed but not real. telebot never validates the token, it
// only uses it as the URL path segment of its first API call, so this token
// reaches the transport and no further.
const validToken = "123456:AAE-characterization-token-not-real"

// deadProxyAddr is 127.0.0.1 port 1, where nothing listens. The socks dialer
// refuses that TCP connect inside the machine, so no test reaches an external
// host and no test depends on what the machine happens to be running.
const deadProxyAddr = "127.0.0.1:1"

// deadProxyURL is deadProxyAddr expressed as a proxy URL.
const deadProxyURL = "socks5://" + deadProxyAddr

// rejectedProxyURL is the smallest proxy URL httpfx refuses by scheme, used
// wherever a case needs a rejected proxy without extra noise in the message.
const rejectedProxyURL = "http://127.0.0.1:8080"

// acceptedProxyURL is an accepted proxy that is never dialled, because every
// case that uses it stops at telegram.New's token gate first.
const acceptedProxyURL = "socks5://127.0.0.1:1080"

// zeroHTTPConfig is the inert literal internal/config hands the graph: the proxy
// URL lives on telegram.Config and is applied per client, so the factory-wide
// config stays empty.
func zeroHTTPConfig() httpfx.Config {
	return httpfx.Config{
		ProxyURL:            "",
		Bypass:              "",
		Timeout:             0,
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 0,
		IdleConnTimeout:     0,
		TLS: httpfx.TLSConfig{
			RootCAFile:          "",
			RootCAPEM:           "",
			RootCAReplaceSystem: false,
		},
	}
}

// caHTTPConfig trusts the fake API's certificate and nothing else, so the only
// name a client can reach is the local one. It is the sole deviation from the
// production httpfx.Config, and it exists only to keep the boot offline.
func caHTTPConfig(caPEM string) httpfx.Config {
	cfg := zeroHTTPConfig()
	cfg.TLS = httpfx.TLSConfig{
		RootCAFile:          "",
		RootCAPEM:           caPEM,
		RootCAReplaceSystem: true,
	}

	return cfg
}

// newGraph builds the production fx graph for one configuration.
//
// The provider order mirrors internal/app.go and internal/config/module.go:
// httpfx.Module() first, then the app's own httpfx.Config (internal/config
// supplies it as a zero literal, and nothing else can - without it httpfx.Factory
// has no configuration to build from), then the telegram config, then the
// telegram module itself.
//
// opts are appended after that wiring, which is how a case adds fx.Populate for
// a type it wants to assert on. The fx event logger is replaced with a nop so a
// failing case reports one assertion instead of a wall of stack traces; the zap
// logger in the graph is the real logger.Module() one, because telegram.Module
// requires it.
func newGraph(cfg telegram.Config, httpCfg httpfx.Config, opts ...fx.Option) *fx.App {
	wiring := []fx.Option{
		logger.Module(),
		httpfx.Module(),
		fx.Provide(func() httpfx.Config { return httpCfg }),
		fx.Supply(cfg),
		telegram.Module(),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
	}

	return fx.New(append(wiring, opts...)...)
}

// graphErr is the construction error of the production graph for one
// configuration.
func graphErr(cfg telegram.Config, httpCfg httpfx.Config, opts ...fx.Option) error {
	return newGraph(cfg, httpCfg, opts...).Err()
}

// TestModule_ProductionGraphResolves boots the real graph and asserts it
// resolves.
//
// WHY THIS TEST EXISTS: it is the regression guard for the duplicate
// *http.Client provider. httpfx.Module() already provides an app-scope
// *http.Client, so any second app-scope provider for that type makes fx.New fail
// with "cannot provide *http.Client from [0]: already provided by
// httpfx.Module.func1". That failure was invisible to go build and to
// golangci-lint and shipped broken until the binary ran. Dropping fx.Private
// from telegram.Module's client provider reproduces it and must fail here; so
// must removing the app's own *http.Client provider.
//
// The only deviation from production is the httpfx.Config payload, which trusts
// the local test CA so the getMe call resolves inside the machine. The modules,
// their order and the private scope are production's.
func TestModule_ProductionGraphResolves(t *testing.T) {
	t.Parallel()

	api := newFakeAPI(t)

	var bot *telebot.Bot

	err := graphErr(
		telegram.Config{Token: validToken, ProxyURL: api.ProxyURL()},
		caHTTPConfig(api.CACertPEM()),
		fx.Populate(&bot),
	)
	if err != nil {
		t.Fatalf("construct the production fx graph: %v", err)
	}

	if bot == nil {
		t.Fatal("the graph resolved but did not populate *telebot.Bot")
	}

	if bot.Me == nil {
		t.Fatal("bot.Me is nil, want the identity served by the local fake API")
	}

	if bot.Me.Username != fakeBotUsername {
		t.Errorf("bot.Me.Username = %q, want %q from the local fake API", bot.Me.Username, fakeBotUsername)
	}
}

// proxyCase is one row of the measured proxy-scheme matrix.
//
// The rows are driven through the real graph with a BLANK token, which turns the
// two possible outcomes into two distinguishable errors without any network:
//
//   - wantAccept: the client provider built a client, so construction proceeded
//     to telegram.New, which stopped at the token gate with ErrInvalidToken.
//   - otherwise: the client provider itself refused the URL, so the graph failed
//     with httpfx.ErrInvalidProxyURL and telegram.New never ran.
//
// wantEcho records whether the rejection quotes the raw configured URL. httpfx
// echoes it whenever it refuses from the raw prefix check, and drops it when
// url.Parse fails after that check passed.
type proxyCase struct {
	name     string
	proxyURL string
	// wantAccept is the measured outcome: true builds a client, false rejects.
	wantAccept bool
	// wantEcho records whether the rejection text repeats the proxy URL.
	wantEcho bool
}

// proxySchemeMatrix is the measured accept/reject matrix of the client provider
// telegram.Module registers (go-core-fx/httpfx v0.2.0), re-measured through the
// production graph after the owner's simplification. Every row records the
// OBSERVED outcome on this branch, not a hypothesis.
//
// Accepted: the empty string (no proxy, the default transport is inherited) and
// well formed socks5 and socks5h URLs. Rejected: everything else, with
// httpfx.ErrInvalidProxyURL intact in the error chain.
//
// Three properties are load bearing and each has a row of its own:
//
//   - "socks5://" with no host is REJECTED ("empty hostname"). The deleted
//     hand-rolled client accepted it and only failed at dial time.
//   - "SOCKS5://" is REJECTED, because httpfx prefix-matches the raw string
//     instead of lowercasing the scheme first.
//   - "socks5://127.0.0.1:99999" is still ACCEPTED: neither httpfx nor the
//     deleted implementation range-checks the port at construction time, so this
//     is unchanged behaviour, not a fresh regression.
var proxySchemeMatrix = []proxyCase{
	{name: "empty string uses the default transport", proxyURL: "", wantAccept: true, wantEcho: false},
	{name: "socks5 with host and port", proxyURL: "socks5://127.0.0.1:1080", wantAccept: true, wantEcho: false},
	{name: "socks5h with host and port", proxyURL: "socks5h://127.0.0.1:1080", wantAccept: true, wantEcho: false},
	{name: "socks5 without port defaults to 1080", proxyURL: "socks5://127.0.0.1", wantAccept: true, wantEcho: false},
	{
		name:       "socks5 with credentials",
		proxyURL:   "socks5://user:pass@127.0.0.1:1080",
		wantAccept: true,
		wantEcho:   false,
	},
	{
		name:       "socks5 with an out of range port is still accepted",
		proxyURL:   "socks5://127.0.0.1:99999",
		wantAccept: true,
		wantEcho:   false,
	},
	{
		name:       "socks5 with no host is rejected",
		proxyURL:   "socks5://",
		wantAccept: false,
		wantEcho:   false,
	},
	{name: "uppercase SOCKS5 is rejected", proxyURL: "SOCKS5://127.0.0.1:1080", wantAccept: false, wantEcho: true},
	{name: "http proxy", proxyURL: "http://127.0.0.1:8080", wantAccept: false, wantEcho: true},
	{name: "https proxy", proxyURL: "https://127.0.0.1:8080", wantAccept: false, wantEcho: true},
	{name: "http proxy with no host", proxyURL: "http://", wantAccept: false, wantEcho: true},
	{name: "uppercase HTTP is rejected", proxyURL: "HTTP://127.0.0.1:8080", wantAccept: false, wantEcho: true},
	{name: "direct scheme is not registered", proxyURL: "direct://127.0.0.1", wantAccept: false, wantEcho: true},
	{name: "socks4 is not registered", proxyURL: "socks4://127.0.0.1:1080", wantAccept: false, wantEcho: true},
	{
		name:       "unix socket scheme is not registered",
		proxyURL:   "unix:///tmp/proxy.sock",
		wantAccept: false,
		wantEcho:   true,
	},
	{name: "unparseable text has no socks5 prefix", proxyURL: "not a url", wantAccept: false, wantEcho: true},
	{
		name:       "host and port without a scheme has no socks5 prefix",
		proxyURL:   "127.0.0.1:1080",
		wantAccept: false,
		wantEcho:   true,
	},
	{
		name:       "missing protocol separator has no socks5 prefix",
		proxyURL:   "://127.0.0.1",
		wantAccept: false,
		wantEcho:   true,
	},
	{
		name:       "unterminated IPv6 literal fails to parse after the prefix check",
		proxyURL:   "socks5://[::1",
		wantAccept: false,
		wantEcho:   false,
	},
	{
		name:       "non numeric port fails to parse after the prefix check",
		proxyURL:   "socks5://127.0.0.1:notaport",
		wantAccept: false,
		wantEcho:   false,
	},
	{
		name:       "leading space fails the prefix check",
		proxyURL:   " socks5://127.0.0.1:1080",
		wantAccept: false,
		wantEcho:   true,
	},
}

// TestModule_ProxySchemeMatrix pins the measured matrix of the client provider
// the production module registers.
func TestModule_ProxySchemeMatrix(t *testing.T) {
	t.Parallel()

	for _, tt := range proxySchemeMatrix {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := graphErr(telegram.Config{Token: "", ProxyURL: tt.proxyURL}, zeroHTTPConfig())
			if err == nil {
				t.Fatalf("graph(%q) resolved, want a rejection at the client provider or the token gate", tt.proxyURL)
			}

			if tt.wantAccept {
				assertAccepted(t, tt.proxyURL, err)

				return
			}

			assertRejected(t, tt.proxyURL, tt.wantEcho, err)
		})
	}
}

// assertAccepted pins that the client provider built a client and the graph only
// stopped at telegram.New's token gate, which is what makes the proxy accepted.
func assertAccepted(t *testing.T, proxyURL string, err error) {
	t.Helper()

	if !errors.Is(err, telegram.ErrInvalidToken) {
		t.Errorf("graph(%q) error %v, want errors.Is ErrInvalidToken", proxyURL, err)
	}

	if errors.Is(err, httpfx.ErrInvalidProxyURL) {
		t.Errorf("graph(%q) error %v, want NOT ErrInvalidProxyURL for an accepted proxy", proxyURL, err)
	}
}

// assertRejected pins that the client provider itself refused the URL: the
// httpfx sentinel is intact, telegram.New never ran, and the message is
// attributable to the client.
func assertRejected(t *testing.T, proxyURL string, wantEcho bool, err error) {
	t.Helper()

	if !errors.Is(err, httpfx.ErrInvalidProxyURL) {
		t.Errorf("graph(%q) error %v, want errors.Is httpfx.ErrInvalidProxyURL", proxyURL, err)
	}

	if errors.Is(err, telegram.ErrInvalidToken) {
		t.Errorf("graph(%q) error %v, want NOT ErrInvalidToken: the client provider rejected before New", proxyURL, err)
	}

	for _, want := range []string{failedClientBuild, "invalid proxy URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("graph(%q) error %q, want it to contain %q", proxyURL, err.Error(), want)
		}
	}

	if !wantEcho {
		return
	}

	if !strings.Contains(err.Error(), proxyURL) {
		t.Errorf("graph(%q) error %q, want it to name the rejected URL", proxyURL, err.Error())
	}
}

// TestModule_ProxyErrorPrecedence pins the three token/proxy precedence cases
// observable through the production graph.
//
// The third case is an accepted contract change, owner decision OD-6: the client
// provider is constructed before telegram.New, so a blank token combined with an
// invalid proxy URL surfaces the proxy rejection instead of the token gate. The
// precedence is deliberately NOT engineered back through the provider.
func TestModule_ProxyErrorPrecedence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		token        string
		proxyURL     string
		wantErrIs    error
		wantNotErrIs error
		// wantContain is a fragment the message must carry; empty skips the check.
		wantContain string
	}{
		{
			name:         "blank token with an accepted proxy yields ErrInvalidToken",
			token:        "",
			proxyURL:     "socks5://127.0.0.1:1080",
			wantErrIs:    telegram.ErrInvalidToken,
			wantNotErrIs: httpfx.ErrInvalidProxyURL,
			wantContain:  "invalid token",
		},
		{
			name:         "valid token with a rejected proxy yields the client rejection",
			token:        validToken,
			proxyURL:     rejectedProxyURL,
			wantErrIs:    httpfx.ErrInvalidProxyURL,
			wantNotErrIs: telegram.ErrInvalidToken,
			wantContain:  failedClientBuild,
		},
		{
			name:         "blank token with a rejected proxy yields the client rejection per OD-6",
			token:        "",
			proxyURL:     rejectedProxyURL,
			wantErrIs:    httpfx.ErrInvalidProxyURL,
			wantNotErrIs: telegram.ErrInvalidToken,
			wantContain:  failedClientBuild,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := graphErr(telegram.Config{Token: tt.token, ProxyURL: tt.proxyURL}, zeroHTTPConfig())
			if err == nil {
				t.Fatalf("graph(token=%q, proxy=%q) resolved, want %v", tt.token, tt.proxyURL, tt.wantErrIs)
			}

			if !errors.Is(err, tt.wantErrIs) {
				t.Errorf("graph(token=%q, proxy=%q) error %v, want errors.Is %v",
					tt.token, tt.proxyURL, err, tt.wantErrIs)
			}

			if errors.Is(err, tt.wantNotErrIs) {
				t.Errorf("graph(token=%q, proxy=%q) error %v, want NOT %v",
					tt.token, tt.proxyURL, err, tt.wantNotErrIs)
			}

			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("graph error %q, want it to contain %q", err.Error(), tt.wantContain)
			}
		})
	}
}

// TestModule_TokenGateRejectsWhitespace pins that the token gate is the only
// check left inside telegram.New, whatever whitespace a token carries, as long
// as the client exists. Proxy validation no longer happens there, so the proxy is
// an accepted one.
func TestModule_TokenGateRejectsWhitespace(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		token string
	}{
		{name: "empty token", token: ""},
		{name: "space padded token", token: "   "},
		{name: "tab and newline token", token: "\t\n "},
		{name: "carriage return only token", token: "\r"},
		{name: "unicode whitespace only token", token: "\u00a0"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := graphErr(telegram.Config{Token: tt.token, ProxyURL: acceptedProxyURL}, zeroHTTPConfig())
			if err == nil {
				t.Fatalf("graph(token=%q) resolved, want ErrInvalidToken", tt.token)
			}

			if !errors.Is(err, telegram.ErrInvalidToken) {
				t.Errorf("graph(token=%q) error %v, want errors.Is ErrInvalidToken", tt.token, err)
			}

			if errors.Is(err, httpfx.ErrInvalidProxyURL) {
				t.Errorf("graph(token=%q) error %v, want NOT ErrInvalidProxyURL", tt.token, err)
			}
		})
	}
}

// TestNew_CreateBotWrapping pins that a token past the gate, combined with an
// accepted but unreachable socks5 proxy, fails inside telebot.NewBot and is
// reported through the "create bot: " operation prefix. The connect is refused
// inside the machine, so this is deterministic and offline, and the named host
// and port in the message prove the request was routed through the socks dialer.
func TestNew_CreateBotWrapping(t *testing.T) {
	t.Parallel()

	err := graphErr(telegram.Config{Token: validToken, ProxyURL: deadProxyURL}, zeroHTTPConfig())
	if err == nil {
		t.Fatal("graph with an unreachable socks5 proxy resolved, want the create bot wrap")
	}

	for _, want := range []string{createBotPrefix, deadProxyAddr, fakeHost + ":443", "connection refused"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("graph error %q, want it to contain %q", err.Error(), want)
		}
	}

	if errors.Is(err, telegram.ErrInvalidToken) {
		t.Errorf("graph error %v, want NOT ErrInvalidToken", err)
	}

	if errors.Is(err, httpfx.ErrInvalidProxyURL) {
		t.Errorf("graph error %v, want NOT ErrInvalidProxyURL: the proxy was accepted", err)
	}
}
