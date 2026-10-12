// Package handler defines the contract every bot handler implements to mount
// its routes on the shared dispatcher.
package handler

import "github.com/capcom6/phone2tg-proxy/internal/bot/router"

// Handler registers bot routes in the shared dispatcher seam. The seam keeps
// the FSM state consultation, the exact-match command predicates and the
// chat-type guard on every route; handlers only declare (state, endpoint)
// pairs and their bodies.
type Handler interface {
	// Register mounts the handler's routes on r.
	Register(r *router.Router) error
}
