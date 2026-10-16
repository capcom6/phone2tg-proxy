package config_test

// telegofx_test.go re-homes the deleted internal/telegramclient suite onto
// the surviving subject: the telegofx.Config production hands telegofx, and
// the proxy client telegofx itself builds from it. The old net/http factory
// module is gone, so the accept/reject matrix, the precedence rows and the
// token gate are measured through the PRODUCTION bot wiring
// (telegofx.Module(WithRouter)), one supplied telegofx.Config per row.
// See graphErr for why the rows supply that value instead of driving the
// TELEGRAM__* env path.
//
// telegofx's rejection sentinels (errUnsupportedProxyScheme, errEmptyProxyHost
// in telegofx/proxy.go) are UNEXPORTED, so errors.Is cannot pin rejections
// the way errors.Is(err, <transport>.ErrInvalidProxyURL) used to. Every
// rejection row therefore asserts the EXACT measured message fragment
// instead, plus the dig constructor attribution; acceptance rows still assert
// errors.Is against telego's exported token sentinels.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/capcom6/phone2tg-proxy/internal/telegramtest"
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

// rejectedProxyURL is rejected by telegofx's scheme check
// (unsupported proxy scheme "http").
const rejectedProxyURL = "http://127.0.0.1:8080"

// failedBotBuild is the dig attribution for a constructor failure: dig names
// the first output type of the failing constructor, telegofx.New, whose
// output is *telegofx.Bot. This is the replacement for the old
// "failed to build []telego.BotOption" prefix the telegramclient provider
// used to produce.
const failedBotBuild = "failed to build *telegofx.Bot"

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
// rows run concurrently.
func graphErr(t *testing.T, token, proxyURL string, botOptions []telego.BotOption, opts ...fx.Option) error {
	t.Helper()

	wiring := []fx.Option{
		logger.Module(),
		fx.Supply(telegofx.Config{
			Token:    token,
			Mode:     telegofx.ModePolling,
			ProxyURL: proxyURL,
		}),
		telegofx.Module(telegofx.WithRouter()),
		fx.Supply(botOptions),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
	}

	return fx.New(append(wiring, opts...)...).Err()
}

// proxyCase is one row of the measured proxy-scheme matrix. Every row
// records the OBSERVED outcome on this branch against telegofx v0.4.0, not a
// hypothesis: a BLANK token turns acceptance into a distinguishable signal,
// because an accepted proxy lets construction proceed to telego's token gate
// (telego.ErrEmptyToken), while a rejected one fails earlier inside
// telegofx.New with a message from NewSocks5Client.
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
// accepts now that telegofx owns the client path (was: the deleted
// internal/telegramclient provider's net/http factory, which prefix-matched
// the raw URL string).
//
// Load-bearing rows:
//
//   - "SOCKS5://127.0.0.1:1080" FLIPS to ACCEPTED (owner-accepted SC-8 change):
//     url.Parse lowercases the scheme before telegofx matches it, where the
//     previous raw-prefix matcher rejected the uppercase form.
//   - "http://..." is still REJECTED, now with telegofx's text
//     (unsupported proxy scheme "http", expected socks5 or socks5h).
//   - "socks5://" with no host is still REJECTED (proxy host is empty).
//   - "socks5://127.0.0.1:99999" is still ACCEPTED: neither implementation
//     range-checks the port at construction time.
//   - Scheme rejections name only the lowercased scheme, NOT the raw URL;
//     parse failures and the empty-host rejection echo the raw URL inside the
//     measured fragment.
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
		wantReject: `proxy host is empty: "socks5://"`,
	},
	{
		name:       "uppercase SOCKS5 is accepted SC-8 flip",
		proxyURL:   "SOCKS5://127.0.0.1:1080",
		wantAccept: true,
		wantReject: "",
	},
	{
		name:       "http proxy",
		proxyURL:   rejectedProxyURL,
		wantAccept: false,
		wantReject: `unsupported proxy scheme "http", expected socks5 or socks5h`,
	},
	{
		name:       "https proxy",
		proxyURL:   "https://127.0.0.1:8080",
		wantAccept: false,
		wantReject: `unsupported proxy scheme "https", expected socks5 or socks5h`,
	},
	{
		name:       "http proxy with no host",
		proxyURL:   "http://",
		wantAccept: false,
		wantReject: `unsupported proxy scheme "http", expected socks5 or socks5h`,
	},
	{
		name:       "uppercase HTTP is rejected with the lowercased scheme",
		proxyURL:   "HTTP://127.0.0.1:8080",
		wantAccept: false,
		wantReject: `unsupported proxy scheme "http", expected socks5 or socks5h`,
	},
	{
		name:       "direct scheme is not registered",
		proxyURL:   "direct://127.0.0.1",
		wantAccept: false,
		wantReject: `unsupported proxy scheme "direct", expected socks5 or socks5h`,
	},
	{
		name:       "socks4 is not registered",
		proxyURL:   "socks4://127.0.0.1:1080",
		wantAccept: false,
		wantReject: `unsupported proxy scheme "socks4", expected socks5 or socks5h`,
	},
	{
		name:       "unix socket scheme is not registered",
		proxyURL:   "unix:///tmp/proxy.sock",
		wantAccept: false,
		wantReject: `unsupported proxy scheme "unix", expected socks5 or socks5h`,
	},
	{
		name:       "unparseable text has no scheme",
		proxyURL:   "not a url",
		wantAccept: false,
		wantReject: `unsupported proxy scheme "", expected socks5 or socks5h`,
	},
	{
		name:       "host and port without a scheme fails to parse",
		proxyURL:   "127.0.0.1:1080",
		wantAccept: false,
		wantReject: `parse proxy url: parse "127.0.0.1:1080": first path segment in URL cannot contain colon`,
	},
	{
		name:       "missing protocol separator fails to parse",
		proxyURL:   "://127.0.0.1",
		wantAccept: false,
		wantReject: `parse proxy url: parse "://127.0.0.1": missing protocol scheme`,
	},
	{
		name:       "unterminated IPv6 literal fails to parse",
		proxyURL:   "socks5://[::1",
		wantAccept: false,
		wantReject: `parse proxy url: parse "socks5://[::1": missing ']' in host`,
	},
	{
		name:       "non numeric port fails to parse",
		proxyURL:   "socks5://127.0.0.1:notaport",
		wantAccept: false,
		wantReject: `parse proxy url: parse "socks5://127.0.0.1:notaport": invalid port ":notaport" after host`,
	},
	{
		name:       "leading space fails to parse",
		proxyURL:   " socks5://127.0.0.1:1080",
		wantAccept: false,
		wantReject: `parse proxy url: parse " socks5://127.0.0.1:1080": first path segment in URL cannot contain colon`,
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

// assertAccepted pins that the proxy client was built (or skipped for "") and
// the graph only stopped at telego's token gate.
func assertAccepted(t *testing.T, proxyURL string, err error) {
	t.Helper()

	if !errors.Is(err, telego.ErrEmptyToken) {
		t.Errorf("graph(%q) error %v, want errors.Is telego.ErrEmptyToken", proxyURL, err)
	}

	if strings.Contains(err.Error(), "create socks5 client") {
		t.Errorf("graph(%q) error %v, want NOT a socks5 client rejection for an accepted proxy", proxyURL, err)
	}
}

// assertRejected pins that telegofx itself refused the URL: the dig
// attribution names the failing constructor, the measured message fragment is
// intact, and telego's token gate never ran. The fragment assertion IS the
// sentinel replacement - telegofx's sentinels are unexported.
func assertRejected(t *testing.T, proxyURL, wantReject string, err error) {
	t.Helper()

	if errors.Is(err, telego.ErrEmptyToken) || errors.Is(err, telego.ErrInvalidToken) {
		t.Errorf("graph(%q) error %v, want NOT a token error: the proxy rejection happens first", proxyURL, err)
	}

	if !strings.Contains(err.Error(), failedBotBuild) {
		t.Errorf("graph(%q) error %q, want it to carry the dig attribution %q", proxyURL, err.Error(), failedBotBuild)
	}

	// Every rejection is wrapped by telegofx.New with its operation prefix
	// before it reaches dig; the rows then pin the sentinel message itself.
	if !strings.Contains(err.Error(), "create socks5 client") {
		t.Errorf("graph(%q) error %q, want it to carry the operation prefix %q",
			proxyURL, err.Error(), "create socks5 client")
	}

	if !strings.Contains(err.Error(), wantReject) {
		t.Errorf("graph(%q) error %q, want it to contain %q", proxyURL, err.Error(), wantReject)
	}
}

// TestConfig_ProxyErrorPrecedence pins the three token/proxy precedence cases
// observable through the production graph. telegofx.New builds the proxy
// client BEFORE telego.NewBot, so a proxy rejection beats the token gate in
// every combination - the OD-6 shape survives the transport swap.
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
			wantContain: `unsupported proxy scheme "http", expected socks5 or socks5h`,
			wantErrIs:   nil,
		},
		{
			name:        "blank token with a rejected proxy yields the proxy rejection per OD-6",
			token:       "",
			proxyURL:    rejectedProxyURL,
			wantContain: `unsupported proxy scheme "http", expected socks5 or socks5h`,
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
// client path: telegofx's fasthttp SOCKS5 client (built from the ProxyURL
// production supplies) -> the loopback SOCKS5 tunnel -> the local fake
// Bot API. No packet leaves the machine - the tunnel ignores the requested
// destination and dials the loopback fake.
//
// Three deviations from production, all test-only: the bot options carry
// telego.WithAPIServer pointing at plain http (telegofx's fasthttp client
// exposes no CA seam, so an offline TLS hop could not be trusted), the
// graph is construction-only - Start is never called, so no polling begins -
// and the telegofx.Config is supplied rather than mapped from the env, as
// graphErr explains.
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
