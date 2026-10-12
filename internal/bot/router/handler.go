package router

import (
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

// HandlerFunc is one state-scoped endpoint handler. The telegohandler
// context carries the bot for replies; the update carries the message the
// dispatcher already validated non-nil before dispatching.
type HandlerFunc func(*th.Context, telego.Update, *StateService) error

// Endpoint keys the dispatcher routes on. Commands keep their literal text
// ("/start", "/stop"); the two catch-all endpoints keep the names of the
// endpoints they replace.
const (
	// OnContact is the key for messages carrying a contact.
	OnContact = "contact"
	// OnText is the key for plain-text fall-through messages.
	OnText = "text"
)
