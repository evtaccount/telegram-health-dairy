package handlers

import (
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	btnComplaints   = "Жалобы"
	btnNoComplaints = "Нет жалоб"
	btnAteNow       = "Поел"
	btnAteAt        = "Поел в …"
	btnChange       = "Изменить"
	btnCancel       = "Отмена"
)

func (h *Handler) HandleCallback(cq *tgbotapi.CallbackQuery) {
	chatID := cq.Message.Chat.ID
	data := cq.Data
	dateKey := extractDateKey(cq.Message.Time(), data)

	// always answer callback
	_, _ = h.Bot.Request(tgbotapi.NewCallback(cq.ID, ""))

	switch {
	case data == cbCfgConfirm:
		h.DB.SetSessionState(chatID, "idle")
		h.send(chatID, "Настройки сохранены! /menu")
	case data == cbCfgChange:
		h.DB.SetUserState(chatID, "setup_morning")
		h.send(chatID, "Введите время утреннего сообщения HH:MM")

	case data == btnComplaints:
		h.DB.SetUserState(chatID, "wait_complaints:"+dateKey)
		h.send(chatID, "Опишите жалобы текстом")
	case data == btnNoComplaints:
		h.DB.UpsertDayRecord(chatID, dateKey[:10], "")
		h.DB.DeletePending(chatID, dateKey)
		h.send(chatID, "Хорошего дня!")
	case data == btnAteNow:
		h.DB.SetDinner(chatID, dateKey[:10], time.Now())
		h.DB.DeletePending(chatID, dateKey)
		h.send(chatID, "Приятного вечера!")
	case data == btnAteAt:
		h.DB.SetUserState(chatID, "wait_dinner:"+dateKey)
		h.send(chatID, "Введите время ужина HH:MM")
	default:
		// ignore others
	}
}

func extractDateKey(t time.Time, data string) string {
	d := t.UTC().Format("2006-01-02")
	if data == btnComplaints || data == btnNoComplaints {
		return d + "-morning"
	}
	return d + "-evening"
}
