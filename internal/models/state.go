package models

type State int

const (
	StateUnknown State = iota
	StateInitial
	StateIdle
	StateMorning
	StateEvening
)
