package bot

import (
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
