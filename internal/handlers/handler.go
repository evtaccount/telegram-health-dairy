package handlers

import (
	"log"

	"telegram-health-dairy/internal/models"
	"telegram-health-dairy/internal/scheduler"
	"telegram-health-dairy/internal/storage"
	"telegram-health-dairy/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	kbYesterdayDinner    = "Вчера ужинал в …"
	kbTodayMorningStatus = "Самочувствие утром"
	kbDinner             = "Ужинал в …"
	kbPrevMorningStatus  = "Самочувствие прошлым утром"
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

func (h *Handler) pushDayKeyboard(chatID int64) {
	st, _ := h.DB.GetSessionState(chatID)
	kb := buildDayKeyboard(st)

	cfg := tgbotapi.NewMessage(chatID, "\u2063") // zero-width char
	cfg.ReplyMarkup = kb
	cfg.DisableNotification = true
	_, _ = h.Bot.Send(cfg)
}

func buildDayKeyboard(st models.State) tgbotapi.ReplyKeyboardMarkup {
	// пустая (но не nil) — Telegram умеет «прятать» клавиатуру,
	// если в ней нет кнопок
	empty := tgbotapi.NewReplyKeyboard()

	switch st {
	case models.StateWaitingMorning, models.StateIdle:
		return tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton(kbYesterdayDinner),
				tgbotapi.NewKeyboardButton(kbTodayMorningStatus),
			),
		)

	case models.StateWaitingEvening:
		return tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton(kbDinner),
				tgbotapi.NewKeyboardButton(kbPrevMorningStatus),
			),
		)
	default: // notStarted, Initial → скрыть
		return empty
	}
}

func NewHandler(bot *tgbotapi.BotAPI, db *storage.DB) *Handler {
	return &Handler{Bot: bot, DB: db}
}
