package models

type Session struct {
	ChatID int64 `db:"chat_id"`
	State  State `db:"state"`
}
