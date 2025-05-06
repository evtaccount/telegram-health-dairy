package handlers

import (
	"telegram-health-dairy/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) HandleCommand(chatID int64, cmd string) {
	switch cmd {
	case "start":
		h.HandleStart(chatID)
	case "settings":
		h.HandleSettings(chatID)
	}
}

// ---------------- /start --------------------
func (h *Handler) HandleStart(chatID int64) {
	u, _ := h.DB.GetUser(chatID)

	if u == nil {
		// create with defaults
		_ = h.DB.UpsertUser(&models.User{
			ChatID:    chatID,
			TZ:        "Europe/Moscow",
			MorningAt: "10:00",
			EveningAt: "18:00",
		})
	}

	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(actionMorning),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(actionEvening),
		),
	)

	reply := tgbotapi.NewMessage(chatID, "Главное меню")
	reply.ReplyMarkup = kb
	_, _ = h.Bot.Send(reply)
}

func (h *Handler) HandleSettings(chatID int64) {
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(menuStats, "show_stat"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(menuMorning, "setup_time_morning"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(menuEvening, "setup_time_evening"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(menuTZ, "setup_timezone"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(menuClear, "clear_stat"),
		),
	)

	reply := tgbotapi.NewMessage(chatID, "Меню настроек")
	reply.ReplyMarkup = kb
	_, _ = h.Bot.Send(reply)
}
