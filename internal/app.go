package internal

import (
	"github.com/capcom6/phone2tg-proxy/internal/bot"
	"github.com/capcom6/phone2tg-proxy/internal/config"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/proxy"
	"github.com/capcom6/phone2tg-proxy/internal/server"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/capcom6/phone2tg-proxy/pkg/telegram"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/healthfx"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/redisfx"
	"github.com/go-core-fx/validatorfx"
	"go.uber.org/fx"
)

func Run() {
	fx.New(
		logger.Module(),
		logger.WithFxDefaultLogger(),
		fiberfx.Module(),
		healthfx.Module(),
		// TODO(F-13): inject real version and build number at release time.
		fx.Provide(func() healthfx.Version {
			return healthfx.Version{
				Version:   "dev",
				ReleaseID: 0,
				BuildDate: "",
				GitCommit: "",
				GoVersion: "",
			}
		}),
		//
		config.Module(),
		telegram.Module(),
		redisfx.Module(),
		validatorfx.Module(),
		//
		storage.Module(),
		server.Module(),
		bot.Module(),
		proxy.Module(),
		i18n.Module(),
	).Run()
}
