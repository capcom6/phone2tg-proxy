package internal

import (
	"github.com/capcom6/phone2tg-proxy/internal/bot"
	"github.com/capcom6/phone2tg-proxy/internal/config"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/proxy"
	"github.com/capcom6/phone2tg-proxy/internal/server"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/capcom6/phone2tg-proxy/internal/telegram"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/healthfx"
	"github.com/go-core-fx/httpfx"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/redisfx"
	"github.com/go-core-fx/telegofx"
	"github.com/go-core-fx/validatorfx"
	"go.uber.org/fx"
)

func Run(version healthfx.Version) {
	fx.New(
		// CORE MODULES
		logger.Module(),
		logger.WithFxDefaultLogger(),
		// badgerfx.Module(),
		// bunfx.Module(),
		// cachefx.Module(),
		fiberfx.Module(),
		// gocqlfx.Module(),
		// gocqlxfx.Module(),
		// goosefx.Module(),
		// gormfx.Module(),
		healthfx.Module(),
		httpfx.Module(),
		// openaifx.Module(),
		// openrouterfx.Module(),
		redisfx.Module(),
		// sqlfx.Module(),
		// sqlxfx.Module(),
		telegofx.Module(telegofx.WithRouter(), telegofx.WithFSM(), telegofx.WithHealth()),
		validatorfx.Module(),
		// watermillfx.Module(),
		//
		// APP MODULES
		config.Module(),
		telegram.Module(),
		//
		// BUSINESS MODULES
		fx.Supply(version),
		storage.Module(),
		server.Module(),
		bot.Module(),
		proxy.Module(),
		i18n.Module(),
	).Run()
}
