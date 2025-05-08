package handlers

import (
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
		u, _ := h.DB.GetUser(chatID)
		newState := calcNextState(u)
		_ = h.DB.SetSessionState(chatID, newState)

		today := time.Now().In(time.UTC).Format("2006-01-02") // дата-ключ

		switch newState {

		case models.StateWaitingMorning:
			// шлём вопрос «Жалобы / Нет жалоб»
			dateKey := today + "-morning"
			msg := tgbotapi.NewMessage(chatID, "Доброе утро! Как самочувствие?")
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
			h.send(chatID, "Настройки сохранены! Ждём ваш ответ 🙂")

		case models.StateWaitingEvening:
			dateKey := today + "-evening"
			hrsLeft := 23 - time.Now().Hour()
			txt := "Пора ужинать! До конца дня осталось " + strconv.Itoa(hrsLeft) + " ч."
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
			h.send(chatID, "Настройки сохранены! Ждём ваш ответ 🙂")

		default: // idle
			h.send(chatID, "Настройки сохранены!\n\n/menu")
		}
	case data == cbCfgChange:
		h.DB.SetUserState(chatID, "setup_morning")
		h.send(chatID, "Введите время утреннего сообщения HH:MM")

	case data == btnComplaints:
		h.DB.SetUserState(chatID, "wait_complaints:"+dateKey)
		h.send(chatID, "Опишите жалобы текстом")
	case data == btnNoComplaints:
		h.DB.UpsertDayRecord(chatID, dateKey[:10], "")
		h.DB.DeletePending(chatID, dateKey)
		h.DB.SetSessionState(chatID, models.StateIdle)
		h.send(chatID, "Хорошего дня!")
	case data == btnAteNow:
		h.DB.SetDinner(chatID, dateKey[:10], time.Now())
		h.DB.DeletePending(chatID, dateKey)
		h.DB.SetSessionState(chatID, models.StateIdle)
		h.send(chatID, "Приятного вечера!")
	case data == btnAteAt:
		h.DB.SetUserState(chatID, "wait_dinner:"+dateKey)
		h.send(chatID, "Введите время ужина HH:MM")
	default:
		// ignore others
	}
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
