package bot

import (
	"fmt"
	"time"
)

// Config is the bot's own transport configuration. config.Module maps it
// from the TELEGRAM__* section as a pure mapping - consumers validate the
// values they use. bot.Module consumes only this type, never the app-wide
// httpfx.Config (which stays at httpfx defaults).
type Config struct {
	// ProxyURL is the SOCKS5 proxy the bot's API traffic dials through.
	ProxyURL string
	// Timeout bounds every outgoing Bot API request end to end.
	Timeout time.Duration
}

// minTelegramTimeout is telego's getUpdates long-poll hold: a client
// timeout at or below it cancels every poll before it can complete, and
// those failed polls still refresh the liveness heartbeat - so liveness
// would stay green while the bot receives nothing. The guard lives here, at
// the consumer, not in config.Module.
const minTelegramTimeout = 8 * time.Second

// Validate checks the config where it is consumed, before it builds the
// bot's HTTP client: the timeout must outlive telego's 8s getUpdates
// long-poll hold (ErrConfigInvalid wraps the detail).
func (c Config) Validate() error {
	if c.Timeout <= minTelegramTimeout {
		return fmt.Errorf(
			"%w: telegram timeout %v must be greater than the 8s getUpdates long-poll hold",
			ErrConfigInvalid, c.Timeout,
		)
	}

	return nil
}
