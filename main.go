package main

import (
	"github.com/capcom6/phone2tg-proxy/internal"
)

//go:generate swag init --parseDependency -g ./main.go -o ./api

//	@title			Phone Number to Telegram Proxy
//	@description	API for sending messages to Telegram by phone number

//	@contact.name	Aleksandr Soloshenko
//	@contact.email	i@capcom.me

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@BasePath		/api/v1

func main() {
	internal.Run()
}
