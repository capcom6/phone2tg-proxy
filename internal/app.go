package internal

import (
	"github.com/capcom6/phone2tg-proxy/internal/bot"
	"github.com/capcom6/phone2tg-proxy/internal/config"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/proxy"
	"github.com/capcom6/phone2tg-proxy/internal/server"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/healthfx"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/redisfx"
	"github.com/go-core-fx/telegofx"
	"github.com/go-core-fx/validatorfx"
	"go.uber.org/fx"
)

func Run(version healthfx.Version) {
	fx.New(
		logger.Module(),
		logger.WithFxDefaultLogger(),
		fiberfx.Module(),
		healthfx.Module(),
		//
		config.Module(),
		// telegofx provides *telegofx.Bot for internal/proxy and the router
		// internal/bot registers its routes on; internal/config supplies its
		// telegofx.Config (token, polling mode, proxy URL) and internal/bot
		// supplies the required []telego.BotOption.
		telegofx.Module(telegofx.WithRouter()),
		redisfx.Module(),
		validatorfx.Module(),
		//
		fx.Supply(version),
		storage.Module(),
		server.Module(),
		bot.Module(),
		proxy.Module(),
		i18n.Module(),
	).Run()
}
