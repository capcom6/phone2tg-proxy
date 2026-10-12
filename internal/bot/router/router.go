package router

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/capcom6/phone2tg-proxy/internal/bot/fsm"
	"github.com/go-core-fx/telegofx"
	tgpred "github.com/go-core-fx/telegofx/predicates"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

// cmdRx is the command grammar the pre-migration dispatcher used: a slash
// command (group 1, including the slash), an optional @bot suffix (group 3),
// then the payload. It mirrors the old update.go:65-77 cmdRx byte for byte.
var cmdRx = regexp.MustCompile(`^(/\w+)(@(\w+))?(\s|$)(.+)?`)

// Router maps (state, endpoint) pairs onto handlers and funnels every
// registered telegohandler route through one shared dispatch function, so
// state resolution behaves identically no matter which route matched.
type Router struct {
	fsm      fsm.FSM
	rt       *telegofx.Router
	username func() string

	handlers   map[fsm.StateName]map[string]HandlerFunc
	registered map[string]struct{}
}

// New builds a dispatcher over rt. username resolves this bot's own username
// for @-suffixed commands; it is called lazily, only when an @suffix is
// actually present, and an empty result rejects every @suffixed command.
func New(f fsm.FSM, rt *telegofx.Router, username func() string) *Router {
	return &Router{
		fsm:      f,
		rt:       rt,
		username: username,

		handlers:   make(map[fsm.StateName]map[string]HandlerFunc),
		registered: make(map[string]struct{}),
	}
}

// handle is the shared dispatch seam: resolve FSM state for the update's
// chat (strconv.FormatInt(chat.ID, 10)), look up handlers[state][endKey],
// return nil when none is registered (SILENT drop), and surface state read
// errors to the caller.
func (r *Router) handle(endKey string, ctx *th.Context, update telego.Update) error {
	if update.Message == nil {
		return nil
	}

	s := NewStateService(r.fsm, strconv.FormatInt(update.Message.Chat.ID, 10))
	state, err := s.Get()
	if err != nil {
		return fmt.Errorf("get state: %w", err)
	}

	if handler := r.handlers[state.Name][endKey]; handler != nil {
		return handler(ctx, update, s)
	}

	return nil
}

// Handle registers h under state+endpoint and, the first time an endpoint
// appears, one telegohandler route whose predicates replicate the old
// endpoint matching and whose body is the shared dispatch above. Route
// registration order is first-match-wins order: commands must register
// before the plain-text fall-through.
func (r *Router) Handle(state fsm.StateName, endpoint string, h HandlerFunc) {
	if r.handlers[state] == nil {
		r.handlers[state] = make(map[string]HandlerFunc)
	}

	if endpoint == "" {
		// unsupported endpoint key
		return
	}
	r.handlers[state][endpoint] = h

	if _, ok := r.registered[endpoint]; !ok {
		r.rt.BaseGroup().Handle(
			func(ctx *th.Context, update telego.Update) error {
				return r.handle(endpoint, ctx, update)
			},
			r.routePredicates(endpoint)...,
		)
		r.registered[endpoint] = struct{}{}
	}
}

// routePredicates builds the predicate chain for one endpoint key. Every
// chain starts with the OD-SC4=(a) chat-type guard: non-private chats are
// dropped before dispatch, so group /start and /stop create no state and
// send nothing.
func (r *Router) routePredicates(endKey string) []th.Predicate {
	guard := tgpred.MessageWithChatType(telego.ChatTypePrivate)

	switch {
	case strings.HasPrefix(endKey, "/"):
		return []th.Predicate{guard, r.commandEqual(endKey)}
	case endKey == OnContact:
		return []th.Predicate{guard, tgpred.MessageWithContact()}
	case endKey == OnText:
		return []th.Predicate{guard, r.anyText()}
	default:
		return []th.Predicate{guard, th.None()}
	}
}

// commandEqual matches ONLY '/<cmd>' or '/<cmd>@<own username>'. The command
// itself is case-sensitive ("/STOP" does not match "/stop"); the @suffix is
// compared EqualFold, mirroring the pre-migration matcher. The username
// provider runs only when an @suffix is present, so a plain '/cmd' never
// triggers a lookup, and an empty username rejects every @suffixed command.
func (r *Router) commandEqual(cmd string) th.Predicate {
	return func(_ context.Context, update telego.Update) bool {
		if update.Message == nil {
			return false
		}

		match := cmdRx.FindStringSubmatch(update.Message.Text)
		if match == nil {
			return false
		}
		if match[1] != cmd {
			return false
		}

		return r.suffixOK(match[3])
	}
}

// anyText is the fall-through to the plain-text endpoint: plain text always
// reaches it; a command reaches it only when no registered command endpoint
// consumed the update (the pre-migration cascade tried commands first); an
// at-sign suffix addressed to another bot drops the update entirely, which
// is a match decision, not a fall-through.
func (r *Router) anyText() th.Predicate {
	return func(_ context.Context, update telego.Update) bool {
		if update.Message == nil || update.Message.Text == "" {
			return false
		}

		match := cmdRx.FindStringSubmatch(update.Message.Text)
		if match == nil {
			return true
		}
		if !r.suffixOK(match[3]) {
			return false
		}

		_, consumed := r.registered[match[1]]
		return !consumed
	}
}

// suffixOK reports whether an @suffix (possibly empty) may address this bot.
// An empty suffix means no @ was sent. EqualFold against an empty username
// (the lazy getMe failed) rejects the command, which is the documented
// degradation.
func (r *Router) suffixOK(suffix string) bool {
	if suffix == "" {
		return true
	}

	return strings.EqualFold(r.username(), suffix)
}
