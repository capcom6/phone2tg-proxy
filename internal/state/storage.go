// Package state provides the in-memory fsm.Storage backing the bot's
// conversation state. It is a deliberate no-behavior-change choice (owner
// decision): no new Redis traffic and no durability change - OD-2 (FSM
// durability) stays open. State is keyed by chat+user pair and never
// expires, matching the superseded chat-keyed FSM, which had no TTL either.
package state

import (
	"context"
	"sync"
)

// Storage keeps fsm states in process memory. It is safe for concurrent use
// and never fails: in-process memory has no failure mode.
type Storage struct {
	mu     sync.RWMutex
	states map[entryKey]string
}

// entryKey identifies one conversation: the same chat+user scheme the
// reference's aiogram-parity keys use ("{chat_id}:{user_id}").
type entryKey struct {
	chatID int64
	userID int64
}

// NewStorage creates an empty in-memory Storage.
func NewStorage() *Storage {
	return &Storage{
		mu:     sync.RWMutex{},
		states: map[entryKey]string{},
	}
}

// Get returns the stored state for the chat/user pair; a missing state
// yields ("", nil). The ctx parameter satisfies the fsm.Storage contract -
// in-memory reads cannot block on it.
func (s *Storage) Get(_ context.Context, chatID, userID int64) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.states[entryKey{chatID: chatID, userID: userID}], nil
}

// Set stores the conversation state for the chat/user pair, replacing
// any previous value.
func (s *Storage) Set(_ context.Context, chatID, userID int64, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.states[entryKey{chatID: chatID, userID: userID}] = state
	return nil
}

// Clear removes the stored state for the chat/user pair. Clearing a
// missing state is not an error.
func (s *Storage) Clear(_ context.Context, chatID, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.states, entryKey{chatID: chatID, userID: userID})
	return nil
}
