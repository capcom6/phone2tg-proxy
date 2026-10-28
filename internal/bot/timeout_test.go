package bot_test

// timeout_test.go pins the consumer-side transport guard: bot.Module
// refuses to build the bot's client from a Telegram timeout that cannot
// outlive telegofx's 8s getUpdates long-poll hold. A shorter bound cancels
// every poll before it completes, and the failed polls still refresh the
// liveness heartbeat - liveness would stay green while the bot receives
// nothing. config.Module only maps the TELEGRAM__* section onto bot.Config
// (pure mapping), so this test drives the whole production path - process
// env -> config.Module -> bot.Config -> the bot's private client provider -
// with the graph shape of graph_test (stubs, construction only, no
// lifecycle hooks, no polling). The test is SERIAL: [testing.T.Setenv]
// panics under [testing.T.Parallel], so the env is set at the top level of
// the test body rather than inside a loop.

import (
	"errors"
	"strings"
	"testing"

	"github.com/capcom6/phone2tg-proxy/internal/bot"
	"github.com/capcom6/phone2tg-proxy/internal/config"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/go-core-fx/httpfx"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/telegofx"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// buildBotConfigGraph runs the production env path through config.Module
// into the bot module's private client provider. The caller owns
// TELEGRAM__TIMEOUT (and TELEGRAM__TOKEN) via t.Setenv.
func buildBotConfigGraph(t *testing.T) error {
	t.Helper()

	return fx.New(
		logger.Module(),
		httpfx.Module(),
		config.Module(),
		telegofx.Module(telegofx.WithRouter(), telegofx.WithFSM()),
		fx.Provide(func() storage.Service { return graphStubStorage{} }),
		fx.Provide(func() (*i18n.Service, error) {
			return i18n.NewService(i18n.Config{
				Language:         "en",
				TranslationsPath: "../../i18n/locales",
			})
		}),
		bot.Module(),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
	).Err()
}

// TestModule_RejectsTelegramTimeoutAtOrBelowLongPollHold drives three env
// values through the production path: below and exactly at the long-poll
// hold must fail construction with the consumer's sentinel, above it must
// build.
func TestModule_RejectsTelegramTimeoutAtOrBelowLongPollHold(t *testing.T) {
	t.Setenv("TELEGRAM__TOKEN", offlineToken)

	t.Setenv("TELEGRAM__TIMEOUT", "5s")
	if err := buildBotConfigGraph(t); err == nil {
		t.Error("graph built with TELEGRAM__TIMEOUT=5s, want the consumer guard to reject it")
	} else {
		if !errors.Is(err, bot.ErrConfigInvalid) {
			t.Errorf("error = %v, want errors.Is ErrConfigInvalid", err)
		}
		if !strings.Contains(err.Error(), "must be greater than the 8s getUpdates long-poll hold") {
			t.Errorf("error = %v, want the wrapped detail about the long-poll hold", err)
		}
		if !strings.Contains(err.Error(), "failed to build *http.Client") {
			t.Errorf("error = %v, want the dig attribution for *http.Client", err)
		}
	}

	t.Setenv("TELEGRAM__TIMEOUT", "8s")
	if err := buildBotConfigGraph(t); err == nil {
		t.Error("graph built with TELEGRAM__TIMEOUT=8s, want the consumer guard to reject it")
	} else if !errors.Is(err, bot.ErrConfigInvalid) {
		t.Errorf("error = %v, want errors.Is ErrConfigInvalid", err)
	}

	t.Setenv("TELEGRAM__TIMEOUT", "9s")
	if err := buildBotConfigGraph(t); err != nil {
		t.Errorf("graph with TELEGRAM__TIMEOUT=9s must build: %v", err)
	}
}
