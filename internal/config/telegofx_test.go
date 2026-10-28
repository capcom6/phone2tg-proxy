package config_test

// telegofx_test.go measures the production bot wiring end to end: the bot
// transport config production maps at config.Module (telegram.Config, ProxyURL
// validation) and the telegofx.Config that rides on top of the client
// telegram.Module builds from it.
// The accept/reject matrix, the precedence rows and the token gate are all
// measured through that graph (an inline mirror of telegram.Module's private
// client seam + telegofx.Module(WithRouter)), one supplied config per row.
// See graphErr for why the rows supply values instead of driving the
// TELEGRAM__* env path.
//
// Rejections surface from httpfx with the EXPORTED sentinel
// httpfx.ErrInvalidProxyURL, so every rejection row asserts errors.Is
// against it plus the EXACT measured message fragment and the dig
// constructor attribution (*http.Client - httpfx builds the client before
// telegofx.New can run); acceptance rows assert errors.Is against telego's
// exported token sentinels.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"time"

	"github.com/capcom6/phone2tg-proxy/internal/telegram"
	"github.com/capcom6/phone2tg-proxy/internal/telegramtest"
	"github.com/go-core-fx/httpfx"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/telegofx"
	"github.com/mymmrac/telego"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// validToken is well formed but not real. telego validates tokens against
// ^\d+:[\w-]{35}$ and never performs a network call at construction, so this
// token satisfies the gate and reaches the transport only if a test asks for
// traffic.
const validToken = "123456:AAE-characterization-token-not-real"

// acceptedProxyURL is an accepted proxy that is never dialled, because
// construction stops at telego's token gate.
const acceptedProxyURL = "socks5://127.0.0.1:1080"

// rejectedProxyURL is rejected by httpfx's scheme prefix check
// (invalid proxy URL: "http://127.0.0.1:8080").
const rejectedProxyURL = "http://127.0.0.1:8080"

// graphHTTPTimeout is the httpfx request bound every row supplies; the rows
// never dial (construction stops at the token gate or earlier), so the value
// only has to be non-zero to keep telegofx's no-timeout warning quiet.
const graphHTTPTimeout = time.Second

// failedClientBuild is the dig attribution for a rejected proxy: dig names
// the first output type of the failing constructor - httpfx's client
// factory, whose output is *[http.Client]. telegofx.New never runs, so the
// proxy rejection cannot carry a token error.
const failedClientBuild = "failed to build *http.Client"

// graphErr constructs the production-shaped graph for one configuration:
// logger + the telegofx.Config under test + telegofx with the router, plus the
// bot options and extra fx options a case needs. Construction only; nothing is
// started, so no polling begins.
//
// The config is SUPPLIED as a value rather than driven through the TELEGRAM__*
// env keys internal/config reads. t.Setenv mutates the process environment and
// is incompatible with t.Parallel, and these rows measure telegofx.New - the
// proxy-scheme matrix, the token gate and their precedence - not the koanf
// mapping that feeds it. Supplying the very values config.Module would have
// produced from the env keeps every row value and every assertion unchanged
// while leaving the process environment untouched, which is what lets the matrix
// rows run concurrently. The consumer's timeout guard (telegram.Module's client
// provider) is likewise not mirrored here: graphHTTPTimeout is 1s by design
// for these construction-only rows, and guard coverage lives in the bot
// package's timeout_test.
func graphErr(t *testing.T, token, proxyURL string, botOptions []telego.BotOption, opts ...fx.Option) error {
	t.Helper()

	wiring := []fx.Option{
		logger.Module(),
		httpfx.Module(),
		fx.Supply(httpfx.Config{}),
		fx.Supply(telegram.Config{
			ProxyURL: proxyURL,
			Timeout:  graphHTTPTimeout,
		}),
		fx.Supply(telegofx.Config{
			Token: token,
			Mode:  telegofx.ModePolling,
		}),
		telegofx.Module(telegofx.WithRouter()),
		// Mirrors telegram.Module as its own fx module: the default
		// httpfx.Factory supplies the base, the telegram module's OWN config
		// overrides the transport per client behind fx.Private (module-scoped,
		// so it shadows httpfx.Module's app-wide client exactly like
		// production), and the exported options provider hands the client to
		// telegofx. The case's telego options (API server, fake caller) ride
		// WithBotOptions.
		fx.Module(
			"telegram",
			fx.Provide(
				func(factory httpfx.Factory, cfg telegram.Config) (*http.Client, error) {
					return factory.NewClient(
						httpfx.WithProxyURL(cfg.ProxyURL, ""),
						httpfx.WithTimeout(cfg.Timeout),
					)
				},
				fx.Private,
			),
			fx.Provide(func(client *http.Client) []telegofx.Option {
				return []telegofx.Option{
					telegofx.WithHTTPClient(client),
					telegofx.WithBotOptions(botOptions...),
				}
			}),
		),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
	}

	return fx.New(append(wiring, opts...)...).Err()
}

// proxyCase is one row of the measured proxy-scheme matrix. Every row
// records the OBSERVED outcome against httpfx (which builds the client), not
// a hypothesis: a BLANK token turns acceptance into a distinguishable
// signal, because an accepted proxy lets construction proceed to telego's
// token gate (telego.ErrEmptyToken), while a rejected one fails earlier in
// httpfx's client factory with httpfx.ErrInvalidProxyURL.
type proxyCase struct {
	name     string
	proxyURL string
	// wantAccept is the measured outcome: true builds a client (or skips the
	// build for the empty string), false rejects.
	wantAccept bool
	// wantReject is the exact rejection fragment telegofx produces. It is
	// empty for accepted rows and is the assertion surface that replaces the
	// unexported sentinels.
	wantReject string
}

// proxySchemeMatrix is the re-measured matrix of what the production config
// accepts now that httpfx owns the client path (was: telegofx building its
// own fasthttp SOCKS5 client; before that, the deleted internal/telegramclient
// provider's net/http factory).
//
// Load-bearing rows:
//
//   - "SOCKS5://127.0.0.1:1080" FLIPS to REJECTED: httpfx prefix-matches the
//     raw URL case-sensitively, so the uppercase form never reaches
//     url.Parse. This REVERSES the earlier owner-accepted SC-8 acceptance.
//   - "http://..." is still REJECTED, now with httpfx's text
//     (invalid proxy URL: "<raw url>").
//   - "socks5://" with no host is still REJECTED (invalid proxy URL: empty
//     hostname).
//   - "socks5://127.0.0.1:99999" is still ACCEPTED: neither implementation
//     range-checks the port at construction time.
//   - Scheme rejections and parse failures echo the RAW URL (httpfx
//     prefix-matches before parsing, and collapses parse failures to the bare
//     sentinel); the empty-host rejection reports the hostname reason.
var proxySchemeMatrix = []proxyCase{
	{name: "empty string uses the default transport", proxyURL: "", wantAccept: true, wantReject: ""},
	{name: "socks5 with host and port", proxyURL: acceptedProxyURL, wantAccept: true, wantReject: ""},
	{name: "socks5h with host and port", proxyURL: "socks5h://127.0.0.1:1080", wantAccept: true, wantReject: ""},
	{name: "socks5 without port defaults to 1080", proxyURL: "socks5://127.0.0.1", wantAccept: true, wantReject: ""},
	{
		name:       "socks5 with credentials",
		proxyURL:   "socks5://user:pass@127.0.0.1:1080",
		wantAccept: true,
		wantReject: "",
	},
	{
		name:       "socks5 with an out of range port is still accepted",
		proxyURL:   "socks5://127.0.0.1:99999",
		wantAccept: true,
		wantReject: "",
	},
	{
		name:       "socks5 with no host is rejected",
		proxyURL:   "socks5://",
		wantAccept: false,
		wantReject: `invalid proxy URL: empty hostname`,
	},
	{
		name:       "uppercase SOCKS5 is rejected case-sensitive prefix SC-8 reversal",
		proxyURL:   "SOCKS5://127.0.0.1:1080",
		wantAccept: false,
		wantReject: `invalid proxy URL: "SOCKS5://127.0.0.1:1080"`,
	},
	{
		name:       "http proxy",
		proxyURL:   rejectedProxyURL,
		wantAccept: false,
		wantReject: `invalid proxy URL: "http://127.0.0.1:8080"`,
	},
	{
		name:       "https proxy",
		proxyURL:   "https://127.0.0.1:8080",
		wantAccept: false,
		wantReject: `invalid proxy URL: "https://127.0.0.1:8080"`,
	},
	{
		name:       "http proxy with no host",
		proxyURL:   "http://",
		wantAccept: false,
		wantReject: `invalid proxy URL: "http://"`,
	},
	{
		name:       "uppercase HTTP is rejected with the raw URL",
		proxyURL:   "HTTP://127.0.0.1:8080",
		wantAccept: false,
		wantReject: `invalid proxy URL: "HTTP://127.0.0.1:8080"`,
	},
	{
		name:       "direct scheme is not registered",
		proxyURL:   "direct://127.0.0.1",
		wantAccept: false,
		wantReject: `invalid proxy URL: "direct://127.0.0.1"`,
	},
	{
		name:       "socks4 is not registered",
		proxyURL:   "socks4://127.0.0.1:1080",
		wantAccept: false,
		wantReject: `invalid proxy URL: "socks4://127.0.0.1:1080"`,
	},
	{
		name:       "unix socket scheme is not registered",
		proxyURL:   "unix:///tmp/proxy.sock",
		wantAccept: false,
		wantReject: `invalid proxy URL: "unix:///tmp/proxy.sock"`,
	},
	{
		name:       "unparseable text has no scheme",
		proxyURL:   "not a url",
		wantAccept: false,
		wantReject: `invalid proxy URL: "not a url"`,
	},
	{
		name:       "host and port without a scheme fails to parse",
		proxyURL:   "127.0.0.1:1080",
		wantAccept: false,
		wantReject: `invalid proxy URL: "127.0.0.1:1080"`,
	},
	{
		name:       "missing protocol separator fails to parse",
		proxyURL:   "://127.0.0.1",
		wantAccept: false,
		wantReject: `invalid proxy URL: "://127.0.0.1"`,
	},
	{
		name:       "unterminated IPv6 literal fails to parse",
		proxyURL:   "socks5://[::1",
		wantAccept: false,
		wantReject: `invalid proxy URL`,
	},
	{
		name:       "non numeric port fails to parse",
		proxyURL:   "socks5://127.0.0.1:notaport",
		wantAccept: false,
		wantReject: `invalid proxy URL`,
	},
	{
		name:       "leading space fails to parse",
		proxyURL:   " socks5://127.0.0.1:1080",
		wantAccept: false,
		wantReject: `invalid proxy URL: " socks5://127.0.0.1:1080"`,
	},
}

// TestConfig_ProxySchemeMatrix pins the measured matrix of the proxy URL the
// production config hands telegofx.
func TestConfig_ProxySchemeMatrix(t *testing.T) {
	t.Parallel()

	for _, tt := range proxySchemeMatrix {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := graphErr(t, "", tt.proxyURL, []telego.BotOption{})
			if err == nil {
				t.Fatalf("graph(%q) resolved, want a rejection at telegofx.New or the token gate", tt.proxyURL)
			}

			if tt.wantAccept {
				assertAccepted(t, tt.proxyURL, err)

				return
			}

			assertRejected(t, tt.proxyURL, tt.wantReject, err)
		})
	}
}

// assertAccepted pins that httpfx built the client (or skipped the build for
// "") and the graph only stopped at telego's token gate.
func assertAccepted(t *testing.T, proxyURL string, err error) {
	t.Helper()

	if !errors.Is(err, telego.ErrEmptyToken) {
		t.Errorf("graph(%q) error %v, want errors.Is telego.ErrEmptyToken", proxyURL, err)
	}

	if errors.Is(err, httpfx.ErrInvalidProxyURL) {
		t.Errorf("graph(%q) error %v, want NOT an httpfx proxy rejection for an accepted proxy", proxyURL, err)
	}
}

// assertRejected pins that httpfx refused the URL: the dig attribution names
// the failing constructor (*http.Client), the exported sentinel matches via
// [errors.Is], and the measured message fragment is intact. telego's token
// gate never ran because telegofx.New never executed.
func assertRejected(t *testing.T, proxyURL, wantReject string, err error) {
	t.Helper()

	if errors.Is(err, telego.ErrEmptyToken) || errors.Is(err, telego.ErrInvalidToken) {
		t.Errorf("graph(%q) error %v, want NOT a token error: the proxy rejection happens first", proxyURL, err)
	}

	if !errors.Is(err, httpfx.ErrInvalidProxyURL) {
		t.Errorf("graph(%q) error %v, want errors.Is httpfx.ErrInvalidProxyURL", proxyURL, err)
	}

	if !strings.Contains(err.Error(), failedClientBuild) {
		t.Errorf(
			"graph(%q) error %q, want it to carry the dig attribution %q",
			proxyURL,
			err.Error(),
			failedClientBuild,
		)
	}

	if !strings.Contains(err.Error(), wantReject) {
		t.Errorf("graph(%q) error %q, want it to contain %q", proxyURL, err.Error(), wantReject)
	}
}

// TestConfig_ProxyErrorPrecedence pins the three token/proxy precedence cases
// observable through the production graph. httpfx builds the client as a
// dependency of telegofx.New, so a proxy rejection surfaces before telego.NewBot
// can run and beats the token gate in every combination - the OD-6 shape
// survives the transport swap.
func TestConfig_ProxyErrorPrecedence(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		token       string
		proxyURL    string
		wantContain string
		// wantErrIs is checked when non-nil; when nil no exported sentinel
		// applies and both token sentinels are asserted ABSENT instead.
		wantErrIs error
	}{
		{
			name:        "blank token with an accepted proxy yields ErrEmptyToken",
			token:       "",
			proxyURL:    acceptedProxyURL,
			wantContain: "telego: empty token",
			wantErrIs:   telego.ErrEmptyToken,
		},
		{
			name:        "valid token with a rejected proxy yields the proxy rejection",
			token:       validToken,
			proxyURL:    rejectedProxyURL,
			wantContain: `invalid proxy URL: "http://127.0.0.1:8080"`,
			wantErrIs:   nil,
		},
		{
			name:        "blank token with a rejected proxy yields the proxy rejection per OD-6",
			token:       "",
			proxyURL:    rejectedProxyURL,
			wantContain: `invalid proxy URL: "http://127.0.0.1:8080"`,
			wantErrIs:   nil,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := graphErr(t, tt.token, tt.proxyURL, []telego.BotOption{})
			if err == nil {
				t.Fatalf("graph(token=%q, proxy=%q) resolved, want a failure", tt.token, tt.proxyURL)
			}

			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("graph(token=%q, proxy=%q) error %q, want it to contain %q",
					tt.token, tt.proxyURL, err.Error(), tt.wantContain)
			}

			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Errorf("graph(token=%q, proxy=%q) error %v, want errors.Is %v",
						tt.token, tt.proxyURL, err, tt.wantErrIs)
				}

				return
			}

			if errors.Is(err, telego.ErrEmptyToken) || errors.Is(err, telego.ErrInvalidToken) {
				t.Errorf("graph(token=%q, proxy=%q) error %v, want NOT a token error: the proxy check runs first",
					tt.token, tt.proxyURL, err)
			}
		})
	}
}

// TestConfig_TokenGateRejectsWhitespace pins that telego's format gate is the
// only check left inside telegofx.New once the proxy is accepted: the blank
// token yields telego.ErrEmptyToken (empty is distinguished from malformed);
// every non-empty malformed token yields telego.ErrInvalidToken, because
// telego validates ^\d+:[\w-]{35}$ instead of trimming.
func TestConfig_TokenGateRejectsWhitespace(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		token   string
		wantErr error
	}{
		{name: "empty token", token: "", wantErr: telego.ErrEmptyToken},
		{name: "space padded token", token: "   ", wantErr: telego.ErrInvalidToken},
		{name: "tab and newline token", token: "\t\n ", wantErr: telego.ErrInvalidToken},
		{name: "carriage return only token", token: "\r", wantErr: telego.ErrInvalidToken},
		{name: "unicode whitespace only token", token: "\u00a0", wantErr: telego.ErrInvalidToken},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := graphErr(t, tt.token, acceptedProxyURL, []telego.BotOption{})
			if err == nil {
				t.Fatalf("graph(token=%q) resolved, want %v", tt.token, tt.wantErr)
			}

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("graph(token=%q) error %v, want errors.Is %v", tt.token, err, tt.wantErr)
			}

			if strings.Contains(err.Error(), "create socks5 client") {
				t.Errorf("graph(token=%q) error %v, want NOT a proxy rejection: the proxy is accepted", tt.token, err)
			}
		})
	}
}

// TestConfig_ProductionGraphGetMeThroughProxy boots the production bot wiring
// with the bot wired in and reads the bot's identity back through the ACTIVE
// client path: httpfx's SOCKS5 client (built from the ProxyURL production
// supplies) -> the loopback SOCKS5 tunnel -> the local fake Bot API. No
// packet leaves the machine - the tunnel ignores the requested destination
// and dials the loopback fake.
//
// Three deviations from production, all test-only: the bot options carry
// telego.WithAPIServer pointing at plain http (the fake cannot present a
// trusted TLS certificate offline), the graph is construction-only - Start is
// never called, so no polling begins - and the configs are supplied rather
// than mapped from the env, as graphErr explains.
func TestConfig_ProductionGraphGetMeThroughProxy(t *testing.T) {
	t.Parallel()

	api := telegramtest.NewFakeAPI(t)

	var b *telegofx.Bot

	err := graphErr(t, validToken, api.ProxyURL(),
		[]telego.BotOption{telego.WithAPIServer(telegramtest.APIServerURL)},
		fx.Populate(&b),
	)
	if err != nil {
		t.Fatalf("construct the production fx graph: %v", err)
	}

	if b == nil {
		t.Fatal("the graph resolved but did not populate *telegofx.Bot")
	}

	me, getErr := b.GetMe(context.Background())
	if getErr != nil {
		t.Fatalf("getMe through the active client path: %v", getErr)
	}

	if me.Username != telegramtest.FakeBotUsername {
		t.Errorf("GetMe username = %q, want %q from the local fake API", me.Username, telegramtest.FakeBotUsername)
	}
}
