package router

import (
	"fmt"

	"github.com/capcom6/phone2tg-proxy/internal/bot/fsm"
)

// StateService provides scoped access to FSM state for a single key (e.g., chat ID).
type StateService struct {
	fsm fsm.FSM
	key string
}

// NewStateService constructs a new per-key state accessor.
func NewStateService(fsm fsm.FSM, key string) *StateService {
	return &StateService{
		fsm: fsm,
		key: key,
	}
}

func (s *StateService) Get() (*fsm.State, error) {
	st, err := s.fsm.Get(s.key)
	if err != nil {
		return nil, fmt.Errorf("get state for key %s: %w", s.key, err)
	}

	return st, nil
}

func (s *StateService) Set(state *fsm.State) error {
	if err := s.fsm.Set(s.key, state); err != nil {
		return fmt.Errorf("set state for key %s: %w", s.key, err)
	}

	return nil
}

func (s *StateService) Delete() error {
	if err := s.fsm.Delete(s.key); err != nil {
		return fmt.Errorf("delete state for key %s: %w", s.key, err)
	}

	return nil
}
