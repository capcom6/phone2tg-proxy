package bot

import (
	"github.com/capcom6/phone2tg-proxy/internal/bot/handler"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers/start"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers/stop"
	"github.com/capcom6/phone2tg-proxy/internal/state"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/telegofx"
	"github.com/go-core-fx/telegofx/fsm"
	"go.uber.org/fx"
)

// Module wires the bot: the fsm store module over an in-memory backing, the
// telegofx option seam, and the start/stop handlers, then registers them on
// the shared telegofx router.
//
// Registration order is load-bearing: telegohandler dispatch is
// first-match-wins over the flat routes slice, and the fx value group
// preserves provide order, so start's routes - including its wait-state
// text fall-through - register BEFORE stop's /stop route. That order
// preserves SC-1/CM1/CM2/CM3 in both directions: the text route only
// matches non-command text while waiting for a contact, and /stop only
// matches StateEmpty, so neither route can shadow the other's inputs.
func Module() fx.Option {
	return fx.Module(
		"bot",
		logger.WithNamedLogger("bot"),
		fx.Provide(
			// OD-2 (FSM durability) stays OPEN: this in-memory backing is a
			// deliberate no-behavior-change choice - no new Redis traffic and
			// no durability change versus the superseded FSM. The
			// implementation lives in internal/fsm/memory so internal/fsm stays
			// implementation-free and extractable into telegofx.
			//
			// This provider must NOT be fx.Private: dig scopes are isolated, and
			// fsm.NewStore lives in the "fsm" module nested under "telegofx",
			// which is a SIBLING of this "bot" module. A private provider here is
			// invisible to it, so *fsm.Store would fail to resolve with
			// "missing type: fsm.Storage" at fx.Invoke time.
			func() fsm.Storage { return state.NewStorage() },
		),
		// go-project-template registration pattern: one subpackage per
		// handler, each constructor annotated into the "handlers" value
		// group and returning handler.Handler. The interface return type is
		// load-bearing - dig keys groups by the EXACT result type, so a
		// concrete return type would make the []handler.Handler slice
		// resolve EMPTY with no error. Provide order (start, then stop)
		// becomes route registration order on the router.
		fx.Provide(
			fx.Annotate(start.New, fx.ResultTags(`group:"handlers"`)),
			fx.Annotate(stop.New, fx.ResultTags(`group:"handlers"`)),
			fx.Private,
		),

		// Handlers register directly on the shared router - the owner's
		// reference pattern; the central registration type is gone.
		fx.Invoke(fx.Annotate(
			func(hs []handler.Handler, router *telegofx.Router) {
				for _, h := range hs {
					h.Register(router)
				}
			},
			fx.ParamTags(`group:"handlers"`),
		)),
	)
}
