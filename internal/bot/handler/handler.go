// Package handler defines the contract every bot handler implements to mount
// its routes directly on the shared telegofx router.
package handler

import "github.com/go-core-fx/telegofx"

// Handler registers bot routes on the shared telegofx router - the owner's
// reference pattern: each handler declares its own ordered predicate chain
// (private-chat guard, fsm.StateEqual, command/text predicate) and
// telegohandler dispatches first-match-wins over registration order. There
// is no central registrar between the handler and the router.
type Handler interface {
	// Register mounts the handler's routes on router.
	Register(router *telegofx.Router)
}
