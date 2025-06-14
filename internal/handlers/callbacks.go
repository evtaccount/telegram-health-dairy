package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"telegram-health-dairy/internal/models"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	btnAteNow = "Поел"
	btnAteAt  = "Поел в …"
	btnChange = "Изменить"
	btnYes    = "Да"
	btnCancel = "Отмена"
)

// одна на весь пакет
var confirmKB = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(btnYes, "cmp_yes"),
		tgbotapi.NewInlineKeyboardButtonData(btnCancel, "cmp_cancel"),
	),
)

// Клавиатура для вечернего вопроса
var eveningKB = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(btnAteNow, btnAteNow),
		tgbotapi.NewInlineKeyboardButtonData(btnAteAt, btnAteAt),
	),
)

func (h *Handler) HandleCallback(cq *tgbotapi.CallbackQuery) {
	chatID := cq.Message.Chat.ID
	data := cq.Data
	dateKey := extractDateKey(cq.Message.Time(), data)

	// always answer callback
	_, _ = h.Bot.Request(tgbotapi.NewCallback(cq.ID, ""))

	switch {
	case data == cbCfgConfirm:
		h.handleConfirmSettings(chatID)
	case data == cbCfgChange:
		h.handleChangeSettings(chatID)
	case data == btnAteNow:
		h.handleAteNow(chatID, dateKey)
	case data == btnAteAt:
		h.handleAteAt(chatID, dateKey)
	case data == btnYes:
		h.handleYes(chatID, cq.Message)
	case data == btnCancel:
		h.handleCancel(chatID)
	}
}

func (h *Handler) handleYes(chatID int64, msg *tgbotapi.Message) {
	// callback.Message.ReplyToMessage содержит исходный текст пользователя
	userText := msg.ReplyToMessage.Text
	dateKey := strings.TrimPrefix(h.mustUserState(chatID), "confirm_complaints:")
	h.DB.UpsertDayRecord(chatID, dateKey[:10], userText)
	h.DB.DeletePending(chatID, dateKey)
	h.DB.SetSessionState(chatID, models.StateIdle)
	h.pushDayKeyboard(chatID)
	h.DB.SetUserState(chatID, "")

	// благодарим
	h.Bot.Send(tgbotapi.NewMessage(chatID, "Спасибо — записал!"))
}

func (h *Handler) handleCancel(chatID int64) {
	dateKey := strings.TrimPrefix(h.mustUserState(chatID), "confirm_complaints:")
	// просим ввести текст заново
	h.DB.SetUserState(chatID, "wait_complaints:"+dateKey)
	h.Bot.Send(tgbotapi.NewMessage(chatID,
		"Хорошо, введите состояние ещё раз текстом"))
}

func (h *Handler) mustUserState(chatID int64) string {
	st, _ := h.DB.GetUserState(chatID)
	return st
}

func (h *Handler) handleConfirmSettings(chatID int64) {
	u, _ := h.DB.GetUser(chatID)
	newState := calcCurrentState(u)
	h.DB.SetSessionState(chatID, newState)
	h.pushDayKeyboard(chatID)

	today := time.Now().In(time.UTC).Format("2006-01-02") // дата-ключ

	h.send(chatID, "Настройки сохранены!")

	switch newState {
	case models.StateWaitingMorning:
		// шлём вопрос «Жалобы / Нет жалоб»
		dateKey := today + "-morning"
		msg := tgbotapi.NewMessage(chatID, "Доброе утро! Опишите своё самочувствие")
		sent, _ := h.Bot.Send(msg)

		// записываем pending + 0 reminded_at
		h.DB.InsertPending(&models.PendingMessage{
			ChatID:  chatID,
			DateKey: dateKey,
			Type:    "morning",
			MsgID:   sent.MessageID,
		})

	case models.StateWaitingEvening:
		dateKey := today + "-evening"
		hrsLeft := 23 - time.Now().Hour()
		txt := "Пора ужинать, до конца дня осталось " + strconv.Itoa(hrsLeft) + " ч."
		msg := tgbotapi.NewMessage(chatID, txt)
		msg.ReplyMarkup = eveningKB
		sent, _ := h.Bot.Send(msg)

		h.DB.InsertPending(&models.PendingMessage{
			ChatID:  chatID,
			DateKey: dateKey,
			Type:    "evening",
			MsgID:   sent.MessageID,
		})
	}

	h.showDebugAllPeriods(chatID, u, newState)
}

func (h *Handler) showDebugAllPeriods(chatID int64, u *models.User, newState models.State) {
	// helper: HH:MM → time.Time, привязанный к сегодняшней дате в loc
	parseHM := func(hm string, loc *time.Location) time.Time {
		t, _ := time.ParseInLocation("15:04", hm, loc)
		now := time.Now().In(loc)
		return time.Date(now.Year(), now.Month(), now.Day(),
			t.Hour(), t.Minute(), 0, 0, loc)
	}

	loc, _ := tzToLocation(u.TZ) // IANA или +03:00
	nowLocal := time.Now().In(loc)

	morningStart := parseHM(u.MorningAt, loc)
	morningEnd := morningStart.Add(2 * time.Hour)
	eveningStart := parseHM(u.EveningAt, loc)
	eveningEnd := eveningStart.Add(2 * time.Hour)

	// вычислим «следующее событие»
	var nextName string
	var nextIn time.Duration

	switch {
	case nowLocal.Before(morningStart):
		nextName = "утреннее окно"
		nextIn = morningStart.Sub(nowLocal)
	case nowLocal.Before(morningEnd):
		nextName = "конец утреннего окна"
		nextIn = morningEnd.Sub(nowLocal)
	case nowLocal.Before(eveningStart):
		nextName = "вечернее окно"
		nextIn = eveningStart.Sub(nowLocal)
	case nowLocal.Before(eveningEnd):
		nextName = "конец вечернего окна"
		nextIn = eveningEnd.Sub(nowLocal)
	default:
		// уже после eveningEnd — следующее утро завтра
		nextName = "завтрашнее утро"
		nextIn = morningStart.Add(24 * time.Hour).Sub(nowLocal)
	}

	debug := fmt.Sprintf(
		"Текущий стейт: %s\n"+
			"UTC: %s\n"+
			"Локальное время: %s (%s)\n\n"+
			"Окно \"утро\": %s — %s\n"+
			"Окно \"вечер\": %s — %s\n\n"+
			"След. событие: %s (через %v)",
		newState,
		time.Now().UTC().Format("15:04:05"),
		nowLocal.Format("15:04:05"), u.TZ,
		morningStart.Format("15:04"), morningEnd.Format("15:04"),
		eveningStart.Format("15:04"), eveningEnd.Format("15:04"),
		nextName, nextIn.Round(time.Minute),
	)

	h.send(chatID, debug)
}

func (h *Handler) handleChangeSettings(chatID int64) {
	h.DB.SetUserState(chatID, "setup_morning")
	h.send(chatID, "Введите время утреннего сообщения HH:MM")
}

func (h *Handler) handleAteNow(chatID int64, dateKey string) {
	h.DB.SetDinner(chatID, dateKey[:10], time.Now())
	h.DB.DeletePending(chatID, dateKey)
	h.DB.SetSessionState(chatID, models.StateIdle)
	h.pushDayKeyboard(chatID)
	h.send(chatID, "Приятного вечера!")
}

func (h *Handler) handleAteAt(chatID int64, dateKey string) {
	h.DB.SetUserState(chatID, "wait_dinner:"+dateKey)
	h.send(chatID, "Введите время ужина HH:MM")
}

// внутри handlers/callbacks.go или рядом
func calcCurrentState(u *models.User) models.State {
	if u == nil { // ← 1. защита от nil
		return models.StateNotStarted //   или Idle — как удобнее
	}
	loc, _ := tzToLocation(u.TZ) // IANA или +03:00 → *time.Location
	now := time.Now().In(loc)

	parse := func(hm string) time.Time {
		t, _ := time.ParseInLocation("15:04", hm, loc)
		return time.Date(now.Year(), now.Month(), now.Day(),
			t.Hour(), t.Minute(), 0, 0, loc)
	}
	morningStart := parse(u.MorningAt)
	eveningStart := parse(u.EveningAt)

	inWindow := func(start time.Time) bool {
		end := start.Add(2 * time.Hour)
		return !now.Before(start) && now.Before(end) // [start, end)
	}

	switch {
	case inWindow(morningStart):
		return models.StateWaitingMorning
	case inWindow(eveningStart):
		return models.StateWaitingEvening
	default:
		return models.StateIdle
	}
}

func extractDateKey(t time.Time, data string) string {
	day := t.UTC().Format("2006-01-02")

	// всё, что связано с ужином, считаем evening
	switch data {
	case btnAteNow, btnAteAt: // «Поел» / «Поел в …»
		return day + "-evening"
	case "cmp_dinner_yes", "cmp_dinner_cancel":
		return day + "-evening"
	}

	// остальное относится к утру (жалобы, cmp_yes/cancel)
	return day + "-morning"
}
