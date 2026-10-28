package config_test

// env_path_test.go restores the koanf env-path coverage that
// telegofx_test.go surrendered when paralleltest forced its rows off
// t.Setenv onto fx.Supply. The matrix rows there measure telegofx.New and
// deliberately do not touch the process environment; this file measures the
// mapping that feeds it and nothing else.
//
// internal/config.Config has UNEXPORTED field types, so an external _test
// package cannot construct one and cannot even name config.New's return value.
// config.Module is the production seam that gets around that: it re-publishes
// the private config as the eight EXPORTED fx configs the rest of the app
// consumes. Populating those eight therefore observes the whole production path
//
//	process env -> go-core-fx/config.Load (koanf) -> config.New -> config.Module -> fx graph
//
// without the unexported type ever being written by hand.
//
// The env keys below are the ones internal/config actually reads, derived from
// the koanf tags in config.go plus the delimiter and lowercasing in
// go-core-fx/config v0.1.0 (env.Provider("__", ...) + envTransform).
// TELEGRAM__MODE is deliberately NOT set: telegramConfig has no Mode field and
// config.Module hardcodes telegofx.ModePolling, so there is no such env key.
// TELEGRAM__TIMEOUT is the bot's request bound (bot.Config, the carrier
// bot.Module consumes); the default 30s is overridden below so a broken
// koanf duration decode is observable. The app-wide httpfx.Config must stay
// at httpfx defaults - the Telegram values are asserted NOT to reach it.

import (
	"slices"
	"testing"
	"time"

	"github.com/capcom6/phone2tg-proxy/internal/bot"
	"github.com/capcom6/phone2tg-proxy/internal/config"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/fiberfx/openapi"
	"github.com/go-core-fx/httpfx"
	"github.com/go-core-fx/redisfx"
	"github.com/go-core-fx/telegofx"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// The env values the row below plants. Every one of them differs from the
// built-in default in config.New, so a broken mapping is observable: the
// defaults are 127.0.0.1:3000, X-Forwarded-For, an empty proxy list,
// OpenAPI enabled, empty public host and path, empty telegram token and proxy
// URL with a 30s default timeout, redis://localhost:6379/0, an empty storage
// secret, "en" and "i18n/locales".
const (
	envToken           = "123456:AAE-env-path-token-not-real"
	envProxyURL        = "socks5h://127.0.0.1:1080"
	envTimeout         = "45s"
	wantTimeout        = 45 * time.Second
	envAddress         = "0.0.0.0:8080"
	envProxyHeader     = "X-Real-IP"
	envProxiesJSON     = `["10.0.0.1","10.0.0.2"]`
	envOpenAPIDisabled = "false"
	envOpenAPIHost     = "api.example.test"
	envOpenAPIPath     = "/openapi"
	envRedisURL        = "redis://127.0.0.1:6380/3"
	envStorageSecret   = "env-path-secret"
	envLanguage        = "de"
	envTranslations    = "custom/locales"
)

// TestModule_EnvPathIsReadThroughKoanf pins that the process environment
// really drives the configuration the app runs with, end to end through the
// production constructor and the production module, and pins every value that
// crosses the boundary - not just the telegram pair. It fails if the koanf
// "__" delimiter, the lowercasing in go-core-fx/config's envTransform, the
// koanf struct tags, the JSON-array coercion of HTTP__PROXIES, the env-over-
// default precedence in config.New, or any of the eight module re-publishers
// breaks.
//
// It is deliberately SERIAL and subtest-free.
//
// Serial: t.Setenv mutates the whole process environment, and
// [testing.T.Setenv] panics under [testing.T.Parallel]. That is safe because Go runs
// every top-level test as a subtest of ONE root test whose body is the loop
// over the test list; a parallel test blocks on the root barrier inside
// t.Parallel and cannot enter its body until that loop has finished, so every
// serial top-level test has already completed. The race detector cannot observe
// an environment race at all, so that barrier - not the detector - is what
// keeps the mutation ordered. The other rows in this package never read the
// environment either: they supply their config values directly.
//
// Subtest-free: paralleltest skips this whole function because it contains
// t.Setenv, but it still requires t.Parallel() inside any t.Run subtest, so
// adding subtests here would cost lint.
func TestModule_EnvPathIsReadThroughKoanf(t *testing.T) {
	t.Setenv("TELEGRAM__TOKEN", envToken)
	t.Setenv("TELEGRAM__PROXY_URL", envProxyURL)
	t.Setenv("TELEGRAM__TIMEOUT", envTimeout)
	t.Setenv("HTTP__ADDRESS", envAddress)
	t.Setenv("HTTP__PROXY_HEADER", envProxyHeader)
	t.Setenv("HTTP__PROXIES", envProxiesJSON)
	t.Setenv("HTTP__OPENAPI__ENABLED", envOpenAPIDisabled)
	t.Setenv("HTTP__OPENAPI__PUBLIC_HOST", envOpenAPIHost)
	t.Setenv("HTTP__OPENAPI__PUBLIC_PATH", envOpenAPIPath)
	t.Setenv("REDIS__URL", envRedisURL)
	t.Setenv("STORAGE__SECRET", envStorageSecret)
	t.Setenv("I18N__DEFAULT_LANGUAGE", envLanguage)
	t.Setenv("I18N__TRANSLATIONS_PATH", envTranslations)

	var (
		telegofxCfg telegofx.Config
		httpfxCfg   httpfx.Config
		botCfg      bot.Config
		fiberfxCfg  fiberfx.Config
		openapiCfg  openapi.Config
		redisfxCfg  redisfx.Config
		storageCfg  storage.Config
		i18nCfg     i18n.Config
	)

	err := fx.New(
		config.Module(),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
		fx.Populate(&telegofxCfg, &httpfxCfg, &botCfg, &fiberfxCfg, &openapiCfg, &redisfxCfg, &storageCfg, &i18nCfg),
	).Err()
	if err != nil {
		t.Fatalf("build the production config graph from the env path: %v", err)
	}

	if telegofxCfg.Token != envToken {
		t.Errorf("telegofx.Config.Token = %q, want the TELEGRAM__TOKEN value %q", telegofxCfg.Token, envToken)
	}

	if botCfg.ProxyURL != envProxyURL {
		t.Errorf("bot.Config.ProxyURL = %q, want the TELEGRAM__PROXY_URL value %q",
			botCfg.ProxyURL, envProxyURL)
	}

	if botCfg.Timeout != wantTimeout {
		t.Errorf("bot.Config.Timeout = %v, want the TELEGRAM__TIMEOUT value %v", botCfg.Timeout, wantTimeout)
	}

	// The app-wide client config must NOT inherit the Telegram transport:
	// those values belong to the bot's private client only.
	if httpfxCfg.ProxyURL != "" {
		t.Errorf("httpfx.Config.ProxyURL = %q, want empty: TELEGRAM__PROXY_URL must not configure the app-wide client",
			httpfxCfg.ProxyURL)
	}

	if httpfxCfg.Timeout != 0 {
		t.Errorf(
			"httpfx.Config.Timeout = %v, want 0 (httpfx default): TELEGRAM__TIMEOUT must not configure the app-wide client",
			httpfxCfg.Timeout,
		)
	}

	if telegofxCfg.Mode != telegofx.ModePolling {
		t.Errorf("telegofx.Config.Mode = %q, want %q: config.Module hardcodes the polling mode",
			telegofxCfg.Mode, telegofx.ModePolling)
	}

	if fiberfxCfg.Address != envAddress {
		t.Errorf("fiberfx.Config.Address = %q, want the HTTP__ADDRESS value %q", fiberfxCfg.Address, envAddress)
	}

	if fiberfxCfg.ProxyHeader != envProxyHeader {
		t.Errorf("fiberfx.Config.ProxyHeader = %q, want the HTTP__PROXY_HEADER value %q",
			fiberfxCfg.ProxyHeader, envProxyHeader)
	}

	wantProxies := []string{"10.0.0.1", "10.0.0.2"}
	if !slices.Equal(fiberfxCfg.Proxies, wantProxies) {
		t.Errorf("fiberfx.Config.Proxies = %v, want the HTTP__PROXIES JSON array %v", fiberfxCfg.Proxies, wantProxies)
	}

	if openapiCfg.Enabled {
		t.Error("openapi.Config.Enabled = true, want false from HTTP__OPENAPI__ENABLED overriding the built-in default")
	}

	if openapiCfg.PublicHost != envOpenAPIHost {
		t.Errorf("openapi.Config.PublicHost = %q, want the HTTP__OPENAPI__PUBLIC_HOST value %q",
			openapiCfg.PublicHost, envOpenAPIHost)
	}

	if openapiCfg.PublicPath != envOpenAPIPath {
		t.Errorf("openapi.Config.PublicPath = %q, want the HTTP__OPENAPI__PUBLIC_PATH value %q",
			openapiCfg.PublicPath, envOpenAPIPath)
	}

	if redisfxCfg.URL != envRedisURL {
		t.Errorf("redisfx.Config.URL = %q, want the REDIS__URL value %q", redisfxCfg.URL, envRedisURL)
	}

	if !slices.Equal(storageCfg.Secret, []byte(envStorageSecret)) {
		t.Errorf("storage.Config.Secret = %q, want the STORAGE__SECRET value %q", storageCfg.Secret, envStorageSecret)
	}

	if i18nCfg.Language != envLanguage {
		t.Errorf("i18n.Config.Language = %q, want the I18N__DEFAULT_LANGUAGE value %q", i18nCfg.Language, envLanguage)
	}

	if i18nCfg.TranslationsPath != envTranslations {
		t.Errorf("i18n.Config.TranslationsPath = %q, want the I18N__TRANSLATIONS_PATH value %q",
			i18nCfg.TranslationsPath, envTranslations)
	}
}
