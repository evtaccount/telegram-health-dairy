package handlers

import (
	"fmt"
	"telegram-health-dairy/internal/models"
	"telegram-health-dairy/internal/utils"
	"time"

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

	if cmd == "reset_all" {
		if err := h.DB.DropAll(); err != nil {
			h.send(chatID, "Ошибка: "+err.Error())
		} else {
			h.send(chatID, "База удалена, перезапустите бот")
		}
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
	case "current_state":
		h.handleCurrentState(chatID)
	case "settings":
		h.handleSettings(chatID)
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

	st, err := h.DB.GetSessionState(chatID)
	utils.LogFor(err)

	// 1. Бот уже запущен и НЕ в Initial-flow  → просто сообщаем состояние.
	if st != models.StateNotStarted && st != models.StateInitial {
		txt := fmt.Sprintf(
			"Бот уже запущен.\n"+
				"Текущий стейт: %s\n\n"+
				"Если хотите изменить настройки, отправьте /settings",
			st,
		)
		h.send(chatID, txt)
	} else {
		// 2. Первая активация или Initial-flow ещё не пройден
		_ = h.DB.SetSessionState(chatID, models.StateInitial)
		h.askConfirmDefaults(chatID)
	}
}

// helpers
func (h *Handler) ensureUser(chatID int64) error {
	user, _ := h.DB.GetUser(chatID)

	if user == nil {
		return h.DB.UpsertUser(&models.User{
			ChatID:    chatID,
			TZ:        "Europe/Moscow",
			MorningAt: "10:00",
			EveningAt: "18:00",
		})
	}
	return nil
}

func (h *Handler) handleCurrentState(chatID int64) {
	_ = h.ensureUser(chatID)
	u, _ := h.DB.GetUser(chatID)
	state := calcCurrentState(u)
	msg := "Текущий статус: " + string(state)
	h.send(chatID, msg)
}

func (h *Handler) handleSettings(chatID int64) {
	u, _ := h.DB.GetUser(chatID)
	tzDisplay := gmtString(u.TZ)

	text := fmt.Sprintf(
		"Текущие настройки:\nУтро: %s\nВечер: %s\nЧасовой пояс: %s",
		u.MorningAt, u.EveningAt, tzDisplay,
	)

	msg := tgbotapi.NewMessage(chatID, text)
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Изменить", cbCfgChange),
			tgbotapi.NewInlineKeyboardButtonData("Отмена", btnCancel),
		),
	)

	msg.ReplyMarkup = kb
	h.Bot.Send(msg)
}

func (h *Handler) send(chatID int64, text string) {
	h.Bot.Send(tgbotapi.NewMessage(chatID, text))
}

func (h *Handler) askConfirmDefaults(chatID int64) {
	u, _ := h.DB.GetUser(chatID)
	tzDisplay := gmtString(u.TZ)

	text := fmt.Sprintf(
		"Текущие настройки:\nУтро: %s\nВечер: %s\nЧасовой пояс: %s",
		u.MorningAt, u.EveningAt, tzDisplay,
	)

	msg := tgbotapi.NewMessage(chatID, text)
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
	isAvailableForAll := cmd == "start" || cmd == "help" || cmd == "current_state"

	if isInitialState && !isAvailableForAll {
		return false
	} else {
		return true
	}
}

func gmtString(tz string) string {
	loc, err := tzToLocation(tz)
	if err != nil || loc == nil { // fallback, чтобы не паниковать
		return "GMT"
	}

	_, off := time.Now().In(loc).Zone()
	if off == 0 {
		return "GMT"
	}
	sign := "+"
	if off < 0 {
		sign = "-"
		off = -off
	}
	h := off / 3600
	m := (off % 3600) / 60
	if m == 0 {
		return fmt.Sprintf("GMT%s%d", sign, h)
	}
	return fmt.Sprintf("GMT%s%02d:%02d", sign, h, m)
}
