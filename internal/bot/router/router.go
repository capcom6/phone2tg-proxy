package router

import (
	"fmt"
	"strconv"

	"github.com/capcom6/phone2tg-proxy/internal/bot/fsm"
	"gopkg.in/telebot.v4"
)

type Router struct {
	fsm fsm.FSM
	bot *telebot.Bot

	handlers   map[fsm.StateName]map[string]HandlerFunc
	registered map[string]struct{}
}

func New(f fsm.FSM, bot *telebot.Bot) *Router {
	return &Router{
		fsm: f,
		bot: bot,

		handlers:   make(map[fsm.StateName]map[string]HandlerFunc),
		registered: make(map[string]struct{}),
	}
}

func (r *Router) handle(end string, c telebot.Context) error {
	chat := c.Chat()
	if chat == nil {
		return nil
	}

	s := NewStateService(r.fsm, strconv.FormatInt(chat.ID, 10))
	state, err := s.Get()
	if err != nil {
		return fmt.Errorf("get state: %w", err)
	}

	if handler := r.handlers[state.Name][end]; handler != nil {
		return handler(c, s)
	}

	return nil
}

func (r *Router) Handle(state fsm.StateName, endpoint interface{}, h HandlerFunc) {
	if r.handlers[state] == nil {
		r.handlers[state] = make(map[string]HandlerFunc)
	}

	endKey := extractEndpoint(endpoint)
	if endKey == "" {
		// unsupported endpoint type
		return
	}
	r.handlers[state][endKey] = h

	if _, ok := r.registered[endKey]; !ok {
		r.bot.Handle(endpoint, func(c telebot.Context) error {
			return r.handle(endKey, c)
		})
		r.registered[endKey] = struct{}{}
	}
}

func extractEndpoint(endpoint interface{}) string {
	switch end := endpoint.(type) {
	case string:
		return end
	case telebot.CallbackEndpoint:
		return end.CallbackUnique()
	}
	return ""
}
