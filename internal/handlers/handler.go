package handlers

import (
	"telegram-health-dairy/internal/scheduler"
	"telegram-health-dairy/internal/storage"
	"telegram-health-dairy/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handler struct {
	Bot *tgbotapi.BotAPI
	DB  *storage.DB
}

func Register(bot *tgbotapi.BotAPI, db *storage.DB) {
	h := &Handler{Bot: bot, DB: db}
	go h.listen() // background

	_, err := scheduler.Start(bot, db)
	utils.Must(err)
}

func (h *Handler) listen() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := h.Bot.GetUpdatesChan(u)

	for upd := range updates {
		switch {
		case upd.Message != nil:
			// === 📌 Обработка текстовых сообщений ===
			h.HandleMessage(upd.Message)

		case upd.CallbackQuery != nil:
			// === 📌 Обработка callback кнопок ===
			h.HandleCallback(upd.CallbackQuery)
		}
	}
}
