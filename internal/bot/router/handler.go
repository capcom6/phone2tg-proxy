package router

import (
	"gopkg.in/telebot.v4"
)

type HandlerFunc func(telebot.Context, *StateService) error
