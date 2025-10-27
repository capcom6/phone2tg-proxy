package fsm

import (
	"encoding/json"
	"fmt"
	"time"
)

// FSM defines state storage operations for the bot's finite state machine.
type FSM interface {
	Get(key string) (*State, error)
	Set(key string, state *State) error
	Delete(key string) error
}

// Service is a JSON-backed FSM implementation over a key-value Storage.
type Service struct {
	prefix string
	exp    time.Duration

	storage Storage
}

func New(prefix string, exp time.Duration, storage Storage) *Service {
	return &Service{
		prefix:  prefix + ":",
		exp:     exp,
		storage: storage,
	}
}

func (f *Service) Get(key string) (*State, error) {
	b, err := f.storage.Get(f.prefix + key)
	if err != nil {
		return nil, fmt.Errorf("get state: %w", err)
	}

	state := new(State)
	if b == nil {
		return NewState(StateEmpty), nil
	}

	if jsonErr := json.Unmarshal(b, state); jsonErr != nil {
		return nil, fmt.Errorf("unmarshal state: %w", jsonErr)
	}

	if state.Data == nil {
		state.Data = make(map[string]string)
	}

	return state, nil
}

func (f *Service) Set(key string, state *State) error {
	b, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if stErr := f.storage.Set(f.prefix+key, b, f.exp); stErr != nil {
		return fmt.Errorf("set state: %w", stErr)
	}

	return nil
}

func (f *Service) Delete(key string) error {
	if err := f.storage.Delete(f.prefix + key); err != nil {
		return fmt.Errorf("delete state: %w", err)
	}

	return nil
}
