package handlers

import (
	"fmt"
	"strconv"
	"telegram-health-dairy/internal/models"
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

// Клавиатура для утреннего вопроса
var morningKB = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(btnComplaints, btnComplaints),
		tgbotapi.NewInlineKeyboardButtonData(btnNoComplaints, btnNoComplaints),
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
	case data == btnComplaints:
		h.handleComplaints(chatID, dateKey)
	case data == btnNoComplaints:
		h.handleNoComplaints(chatID, dateKey)
	case data == btnAteNow:
		h.handleAteNow(chatID, dateKey)
	case data == btnAteAt:
		h.handleAteAt(chatID, dateKey)
	}
}

func (h *Handler) handleConfirmSettings(chatID int64) {
	u, _ := h.DB.GetUser(chatID)
	newState := calcNextState(u)
	_ = h.DB.SetSessionState(chatID, newState)

	today := time.Now().In(time.UTC).Format("2006-01-02") // дата-ключ

	h.showDebugAllPeriods(chatID, u, newState)

	switch newState {
	case models.StateWaitingMorning:
		// шлём вопрос «Жалобы / Нет жалоб»
		dateKey := today + "-morning"
		msg := tgbotapi.NewMessage(chatID, "Настройки сохранены! Как самочувствие?")
		msg.ReplyMarkup = morningKB // inline-кнопки
		sent, _ := h.Bot.Send(msg)

		// записываем pending + 0 reminded_at
		h.DB.InsertPending(&models.PendingMessage{
			ChatID:    chatID,
			DateKey:   dateKey,
			Type:      "morning",
			MsgID:     sent.MessageID,
			CreatedAt: time.Now().Unix(),
		})

	case models.StateWaitingEvening:
		dateKey := today + "-evening"
		hrsLeft := 23 - time.Now().Hour()
		txt := "Настройки сохранены! Пора ужинать, до конца дня осталось " + strconv.Itoa(hrsLeft) + " ч."
		msg := tgbotapi.NewMessage(chatID, txt)
		msg.ReplyMarkup = eveningKB
		sent, _ := h.Bot.Send(msg)

		h.DB.InsertPending(&models.PendingMessage{
			ChatID:    chatID,
			DateKey:   dateKey,
			Type:      "evening",
			MsgID:     sent.MessageID,
			CreatedAt: time.Now().Unix(),
		})

	default:
		h.send(chatID, "Настройки сохранены!")
	}
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
			"Локальное (%s): %s\n\n"+
			"Окно утро   : %s — %s\n"+
			"Окно вечер  : %s — %s\n\n"+
			"След. событие: %s (через %v)",
		newState,
		time.Now().UTC().Format("15:04:05"),
		u.TZ, nowLocal.Format("15:04:05"),
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

func (h *Handler) handleComplaints(chatID int64, dateKey string) {
	h.DB.SetUserState(chatID, "wait_complaints:"+dateKey)
	h.send(chatID, "Опишите жалобы текстом")
}

func (h *Handler) handleNoComplaints(chatID int64, dateKey string) {
	h.DB.UpsertDayRecord(chatID, dateKey[:10], "")
	h.DB.DeletePending(chatID, dateKey)
	h.DB.SetSessionState(chatID, models.StateIdle)
	h.send(chatID, "Хорошего дня!")
}

func (h *Handler) handleAteNow(chatID int64, dateKey string) {
	h.DB.SetDinner(chatID, dateKey[:10], time.Now())
	h.DB.DeletePending(chatID, dateKey)
	h.DB.SetSessionState(chatID, models.StateIdle)
	h.send(chatID, "Приятного вечера!")
}

func (h *Handler) handleAteAt(chatID int64, dateKey string) {
	h.DB.SetUserState(chatID, "wait_dinner:"+dateKey)
	h.send(chatID, "Введите время ужина HH:MM")
}

// внутри handlers/callbacks.go или рядом
func calcNextState(u *models.User) models.State {
	loc, err := time.LoadLocation(u.TZ)
	if err != nil {
		// fallback – UTC
		loc = time.UTC
	}
	now := time.Now().In(loc)

	// parse HH:MM
	parse := func(hm string) time.Time {
		t, _ := time.ParseInLocation("15:04", hm, loc)
		// привязываем к сегодняшней дате
		return time.Date(now.Year(), now.Month(), now.Day(),
			t.Hour(), t.Minute(), 0, 0, loc)
	}
	morningStart := parse(u.MorningAt)
	eveningStart := parse(u.EveningAt)

	if now.After(morningStart) && now.Before(morningStart.Add(2*time.Hour)) {
		return models.StateWaitingMorning
	}
	if now.After(eveningStart) && now.Before(eveningStart.Add(2*time.Hour)) {
		return models.StateWaitingEvening
	}
	return models.StateIdle
}

func extractDateKey(t time.Time, data string) string {
	d := t.UTC().Format("2006-01-02")
	if data == btnComplaints || data == btnNoComplaints {
		return d + "-morning"
	}
	return d + "-evening"
}
