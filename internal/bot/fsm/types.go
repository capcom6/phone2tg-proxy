package fsm

import "maps"

type StateName string

const StateEmpty StateName = ""

type State struct {
	Name StateName         `json:"name"`
	Data map[string]string `json:"data"`
}

func NewState(name StateName) *State {
	return &State{
		Name: name,
		Data: make(map[string]string),
	}
}

func (s *State) Clone() *State {
	data := map[string]string{}
	maps.Copy(data, s.Data)

	return &State{
		Name: s.Name,
		Data: data,
	}
}
