package handlers

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"telegram-health-dairy/internal/utils"
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
		h.send(chatID, "Введите часовой пояс (например Europe/Moscow или +3, -05:30, UTC)")

	case state == "setup_timezone":
		tz, err := validateTZ(msg.Text)

		if err != nil {
			utils.LogFor(err)
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

var offRx = regexp.MustCompile(`^(?i)(?:gmt|utc)?([+-]\d{1,2})(?::?(\d{2}))?$`)

func validateTZ(input string) (string, error) {
	tz, err := parseUserTZ(input)
	log.Println(fmt.Printf("Parsed tz: %s", tz))

	if err != nil {
		return "", err
	}

	if _, err := time.LoadLocation(tz); err != nil {
		return "", err
	}

	return tz, nil
}

func parseUserTZ(input string) (string, error) {
	input = strings.TrimSpace(input)

	// 1. пробуем как IANA-имя
	if _, err := time.LoadLocation(input); err == nil {
		return input, nil
	}

	// 2. пробуем как смещение
	m := offRx.FindStringSubmatch(strings.ToUpper(input))
	if m == nil {
		return "", errors.New("не распознан формат: пример +3, -05:30 или Europe/Moscow")
	}

	hPart := m[1]   // "+3"  или "-05"
	minPart := m[2] // ""   или "30"

	if minPart == "" {
		minPart = "00"
	}
	// Нормализуем к формату "+03:00"
	h, _ := strconv.Atoi(hPart)
	return fmt.Sprintf("%+03d:%s", h, minPart), nil
}
