package handlers

import (
	"fmt"
	"telegram-health-dairy/internal/models"
	"telegram-health-dairy/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	menuStats   = "Показать статистику"
	menuMorning = "Задать время утреннего сообщения"
	menuEvening = "Задать время вечернего сообщения"
	menuTZ      = "Сменить часовой пояс"
	menuClear   = "Очистить данные"
)

const (
	cbCfgConfirm = "cfg_confirm"
	cbCfgChange  = "cfg_change"
)

func (h *Handler) HandleCommand(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	cmd := msg.Command()
	st, _ := h.DB.GetSessionState(chatID)

	if cmd == "reset" {
		h.DB.ClearData(chatID)
	}

	if !validateInitialState(st, cmd) {
		if st == models.StateInitial {
			h.send(chatID, "Перед тем как продолжить работу с ботом, подтвердите настройки")
		}
		return
	}

	switch cmd {
	case "start":
		h.handleStart(chatID)
	case "help":
		h.send(chatID, "/start — начать\n/help — справка")
	default:
		// main menu buttons
		switch msg.Text {
		case menuStats:
			h.send(chatID, "Статистика пока не реализована")
		case menuMorning:
			_ = h.DB.SetUserState(chatID, "setup_morning")
			h.send(chatID, "Введите время утреннего сообщения HH:MM")
		case menuEvening:
			_ = h.DB.SetUserState(chatID, "setup_evening")
			h.send(chatID, "Введите время вечернего сообщения HH:MM")
		case menuTZ:
			_ = h.DB.SetUserState(chatID, "setup_timezone")
			h.send(chatID, "Введите часовой пояс, например Europe/Moscow")
		case menuClear:
			_ = h.DB.ClearData(chatID)
			h.send(chatID, "Данные очищены")
		}
	}
}

func (h *Handler) handleStart(chatID int64) {
	err := h.ensureUser(chatID)
	utils.LogFor(err)

	err = h.DB.SetSessionState(chatID, models.StateInitial)
	utils.LogFor(err)

	h.askConfirmDefaults(chatID)
}

// helpers
func (h *Handler) ensureUser(chatID int64) error {
	user, _ := h.DB.GetUser(chatID)

	if user == nil {
		return h.DB.UpsertUser(&models.User{
			ChatID:    chatID,
			TZ:        "Local",
			MorningAt: "10:00",
			EveningAt: "18:00",
		})
	}
	return nil
}

func (h *Handler) send(chatID int64, text string) {
	h.Bot.Send(tgbotapi.NewMessage(chatID, text))
}

func (h *Handler) askConfirmDefaults(chatID int64) {
	u, _ := h.DB.GetUser(chatID)

	msg := tgbotapi.NewMessage(
		chatID,
		fmt.Sprintf("Текущие настройки:\nУтро: %s\nВечер: %s\nTZ: %s", u.MorningAt, u.EveningAt, u.TZ),
	)
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Подтвердить", cbCfgConfirm),
			tgbotapi.NewInlineKeyboardButtonData("Изменить", cbCfgChange),
		),
	)
	msg.ReplyMarkup = kb
	h.Bot.Send(msg)
}

func validateInitialState(st models.State, cmd string) bool {
	isInitialState := (st == models.StateNotStarted) || (st == models.StateInitial)
	isAvailableForAll := cmd == "start" || cmd == "help"

	if isInitialState && !isAvailableForAll {
		return false
	} else {
		return true
	}
}
