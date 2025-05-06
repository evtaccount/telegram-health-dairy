package handlers

import (
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) HandleCallback(cq *tgbotapi.CallbackQuery) {
	data := cq.Data
	chatID := cq.Message.Chat.ID
	dateKey := extractDateKey(cq.Message.Time(), data) // helper below

	switch data {
	case btnComplaints, btnNoComplaints:
		rec, _ := h.DB.GetDayRecord(chatID, dateKey[:10])
		if rec != nil && rec.Complaints != "" {
			// already answered -> ask confirmation
			kb := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(btnChange, "chg:"+data),
					tgbotapi.NewInlineKeyboardButtonData(btnCancel, btnCancel),
				),
			)
			_, _ = h.Bot.Request(tgbotapi.NewEditMessageReplyMarkup(chatID, cq.Message.MessageID, kb))
			return
		}
		if data == btnComplaints {
			_ = h.DB.SetUserState(chatID, "wait_complaints:"+dateKey)
			_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Опиши жалобы текстом"))
		} else {
			_ = h.DB.UpsertDayRecord(chatID, dateKey[:10], "")
			_ = h.DB.DeletePending(chatID, dateKey)
			_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Хорошего дня!"))
		}

	case btnAteNow:
		now := time.Now()
		_ = h.DB.SetDinner(chatID, dateKey[:10], now)
		_ = h.DB.DeletePending(chatID, dateKey)
		_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Приятного вечера!"))

	case btnAteAt:
		_ = h.DB.SetUserState(chatID, "wait_dinner_time:"+dateKey)
		_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Во сколько поужинал? (HH:MM)"))

	case btnCancel:
		h.Bot.Request(tgbotapi.NewCallback(cq.ID, "Отменено"))
	}

	// always answer callback to remove 'loading...'
	h.Bot.Request(tgbotapi.NewCallback(cq.ID, ""))
}

func extractDateKey(t time.Time, typ string) string {
	d := t.UTC().Format("2006-01-02")
	if strings.HasPrefix(typ, "Жалоб") || strings.HasPrefix(typ, "chg:") {
		return d + "-morning"
	}
	return d + "-evening"
}
