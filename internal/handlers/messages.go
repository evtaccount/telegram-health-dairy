package handlers

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var timeRx = regexp.MustCompile(`^\d{1,2}:\d{2}$`)

func (h *Handler) HandleMessage(msg *tgbotapi.Message) {
	if msg.IsCommand() {
		h.HandleCommand(msg)
	} else {
		h.HandleText(msg)
	}
}

func (h *Handler) HandleText(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	state, _ := h.DB.GetUserState(chatID)
	if state == "" {
		return
	}
	switch {
	case state == "setup_morning":
		if !timeRx.MatchString(msg.Text) {
			h.send(chatID, "Неверный формат, нужно HH:MM")
			return
		}
		u, _ := h.DB.GetUser(chatID)
		u.MorningAt = msg.Text
		_ = h.DB.UpsertUser(u)
		_ = h.DB.SetUserState(chatID, "setup_evening")
		h.send(chatID, "Введите время вечернего сообщения HH:MM")

	case state == "setup_evening":
		if !timeRx.MatchString(msg.Text) {
			h.send(chatID, "Неверный формат, нужно HH:MM")
			return
		}
		u, _ := h.DB.GetUser(chatID)
		u.EveningAt = msg.Text
		_ = h.DB.UpsertUser(u)
		_ = h.DB.SetUserState(chatID, "setup_timezone")
		h.send(chatID, "Введите часовой пояс, например Europe/Moscow")

	case state == "setup_timezone":
		tz := msg.Text
		if _, err := time.LoadLocation(tz); err != nil {
			h.send(chatID, "Неверный TZ")
			return
		}
		u, _ := h.DB.GetUser(chatID)
		u.TZ = tz
		_ = h.DB.UpsertUser(u)
		_ = h.DB.SetUserState(chatID, "")
		h.DB.SetSessionState(chatID, "idle")
		h.send(chatID, "Настройки сохранены!")

	case strings.HasPrefix(state, "wait_complaints:"):
		dateKey := strings.TrimPrefix(state, "wait_complaints:")
		h.DB.UpsertDayRecord(chatID, dateKey[:10], msg.Text)
		h.DB.DeletePending(chatID, dateKey)
		_ = h.DB.SetUserState(chatID, "")
		h.send(chatID, "Жалобы сохранены!")

	case strings.HasPrefix(state, "wait_dinner:"):
		if !timeRx.MatchString(msg.Text) {
			h.send(chatID, "Неверный формат, нужно HH:MM")
			return
		}
		parts := strings.Split(msg.Text, ":")
		hStr, _ := strconv.Atoi(parts[0])
		mStr, _ := strconv.Atoi(parts[1])
		now := time.Now()
		dinner := time.Date(now.Year(), now.Month(), now.Day(), hStr, mStr, 0, 0, time.UTC)
		dateKey := strings.TrimPrefix(state, "wait_dinner:")
		h.DB.SetDinner(chatID, dateKey[:10], dinner)
		h.DB.DeletePending(chatID, dateKey)
		_ = h.DB.SetUserState(chatID, "")
		h.send(chatID, "Время ужина сохранено!")
	}
}
