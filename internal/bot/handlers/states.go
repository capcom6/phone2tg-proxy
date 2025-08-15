package handlers

import "github.com/capcom6/phone2tg-proxy/internal/bot/fsm"

const (
	// StateStartWaitForContact is the FSM state entered after /start, awaiting user's contact.
	StateStartWaitForContact fsm.StateName = "start:wait_for_contact"
)
