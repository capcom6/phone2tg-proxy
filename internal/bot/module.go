package bot

import (
	"fmt"

	"github.com/capcom6/phone2tg-proxy/internal/bot/fsm"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handler"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers/start"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers/stop"
	"github.com/capcom6/phone2tg-proxy/internal/bot/router"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/telegofx"
	"github.com/gofiber/storage/memory/v2"
	"github.com/mymmrac/telego"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"bot",
		logger.WithNamedLogger("bot"),
		fx.Provide(
			func() fsm.Storage {
				return memory.New()
			},
			func(storage fsm.Storage) *fsm.Service {
				return fsm.New("fsm", 0, storage)
			},
			// The dispatcher seam is shared by every handler: it owns the
			// (state, endpoint) registry, the FSM state consultation and the
			// exact-match command predicates the routes run through. The
			// bot.Username argument is a bound method value: telego resolves
			// the username at most once, lazily on the first @-suffixed
			// command, never while the fx graph is being constructed
			// (OD-C=a: no getMe at startup). If the lookup fails, the
			// username is empty and every @suffixed command is rejected.
			func(f *fsm.Service, rt *telegofx.Router, bot *telegofx.Bot) *router.Router {
				return router.New(f, rt, bot.Username)
			},
			fx.Private,
		),
		// go-project-template registration pattern: one subpackage per
		// handler, each constructor annotated into the "handlers" value
		// group and returning handler.Handler. The interface return type is
		// load-bearing - dig keys groups by the EXACT result type, so a
		// concrete return type would make the []handler.Handler slice
		// resolve EMPTY with no error.
		fx.Provide(
			fx.Annotate(start.New, fx.ResultTags(`group:"handlers"`)),
			fx.Annotate(stop.New, fx.ResultTags(`group:"handlers"`)),
			fx.Private,
		),
		// telegofx.New requires a []telego.BotOption value in the graph. No
		// client option is needed here: the proxy client is built inside
		// telegofx from telegofx.Config.ProxyURL and the zap logger is
		// appended by telegofx itself.
		fx.Provide(func() []telego.BotOption {
			return []telego.BotOption{}
		}),
		fx.Invoke(fx.Annotate(
			func(handlers []handler.Handler, r *router.Router) error {
				for _, h := range handlers {
					if err := h.Register(r); err != nil {
						return fmt.Errorf("register handler %T: %w", h, err)
					}
				}

				return nil
			},
			fx.ParamTags(`group:"handlers"`),
		)),
	)
}
