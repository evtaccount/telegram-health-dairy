package models

type State string

const (
	StateNotStarted     State = "notStarted"
	StateInitial        State = "initial"
	StateWaitingMorning State = "waiting_morning"
	StateWaitingEvening State = "waiting_evening"
	StateIdle           State = "idle"
)
