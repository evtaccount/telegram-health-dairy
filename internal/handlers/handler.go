package handlers

import (
	"log"

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
	utils.LogFor(err)
}

func (h *Handler) listen() {
	go func() {
		// было: scheduler.Start(h, h.DB)
		if _, err := scheduler.Start(h.Bot, h.DB); err != nil {
			log.Fatal(err)
		}
	}()

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

func NewHandler(bot *tgbotapi.BotAPI, db *storage.DB) *Handler {
	return &Handler{Bot: bot, DB: db}
}
