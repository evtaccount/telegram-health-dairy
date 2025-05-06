package handlers

import (
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) HandleText(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	state, _ := h.DB.GetUserState(chatID)
	if state == "" {
		return
	}

	switch {
	case strings.HasPrefix(state, "wait_complaints:"):
		dateKey := strings.TrimPrefix(state, "wait_complaints:")
		_ = h.DB.UpsertDayRecord(chatID, dateKey[:10], msg.Text)
		_ = h.DB.DeletePending(chatID, dateKey)
		_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Жалобы записаны."))
	case strings.HasPrefix(state, "wait_dinner_time:"):
		if !timeRx.MatchString(msg.Text) {
			_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Формат HH:MM"))
			return
		}
		tparts := strings.Split(msg.Text, ":")
		hStr, _ := strconv.Atoi(tparts[0])
		mStr, _ := strconv.Atoi(tparts[1])
		now := time.Now()
		dt := time.Date(now.Year(), now.Month(), now.Day(), hStr, mStr, 0, 0, time.UTC)
		dateKey := strings.TrimPrefix(state, "wait_dinner_time:")
		_ = h.DB.SetDinner(chatID, dateKey[:10], dt)
		_ = h.DB.DeletePending(chatID, dateKey)
		_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Записал, спасибо!"))
	}
	_ = h.DB.SetUserState(chatID, "")
}
