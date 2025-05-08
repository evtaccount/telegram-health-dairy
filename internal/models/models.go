package models

import "time"

// User represents bot settings for a telegram user.
type User struct {
	ID        int64  `db:"id"         json:"id"`
	ChatID    int64  `db:"chat_id"    json:"chat_id"`
	TZ        string `db:"tz"         json:"tz"`
	MorningAt string `db:"morning_at" json:"morning_at"` // "HH:MM"
	EveningAt string `db:"evening_at" json:"evening_at"` // "HH:MM"
	CreatedAt int64  `db:"created_at" json:"created_at"`
}

// DayRecord stores daily complaints & dinner info.
type DayRecord struct {
	ID         int64      `db:"id"`
	ChatID     int64      `db:"chat_id"`
	Day        string     `db:"day"`                 // YYYY-MM-DD
	Complaints string     `db:"complaints"`          // empty -> no complaints
	DinnerAt   *time.Time `db:"dinner_at,omitempty"` // nil -> not set
}

// PendingMessage tracks messages waiting for reply.
type PendingMessage struct {
	ID         int64  `db:"id"`
	ChatID     int64  `db:"chat_id"`
	DateKey    string `db:"date_key"`    // 2025-05-08-morning / …-evening
	Type       string `db:"type"`        // "morning" либо "evening"
	MsgID      int    `db:"msg_id"`      // ID исходного сообщения-вопроса
	CreatedAt  int64  `db:"created_at"`  // когда вопрос был отправлен
	RemindedAt int64  `db:"reminded_at"` // когда в последний раз напомнили (0 = ещё не напоминали)
}

// UserState stores transient FSM states (waiting text input)
type UserState struct {
	ChatID int64  `db:"chat_id"`
	State  string `db:"state"`
}
