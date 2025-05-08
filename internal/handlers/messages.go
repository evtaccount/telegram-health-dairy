package handlers

import (
	"errors"
	"fmt"
	"log"
	"math"
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

		h.handleConfirmSettings(chatID)

	case strings.HasPrefix(state, "wait_complaints:"):
		dateKey := strings.TrimPrefix(state, "wait_complaints:")

		// 1) шлём сообщение-подтверждение реплаем на текст пользователя
		confirm := tgbotapi.NewMessage(chatID, "Сохраняем текущий статус?")
		confirm.ReplyToMessageID = msg.MessageID
		confirm.ReplyMarkup = confirmKB
		_, _ = h.Bot.Send(confirm)

		// 2) переключаемся в состояние "confirm_complaints:DATE"
		_ = h.DB.SetUserState(chatID, "confirm_complaints:"+dateKey)

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
	if err != nil {
		return "", err
	}
	// Попробуем получить *time.Location — и IANA, и "+03:00"
	if _, err := tzToLocation(tz); err != nil {
		return "", err
	}
	return tz, nil
}

func parseUserTZ(input string) (string, error) {
	input = strings.TrimSpace(input)
	log.Printf("Timezone input: @%s", input)

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
	log.Printf("Timezone parsing result: @%s", fmt.Sprintf("%+03d:%s", h, minPart))
	return fmt.Sprintf("%+03d:%s", h, minPart), nil
}

// tzToLocation строит *time.Location из IANA-имени или смещения "+03:00".
func tzToLocation(tz string) (*time.Location, error) {
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc, nil // IANA ok
	}
	m := offRx.FindStringSubmatch(strings.ToUpper(tz))
	if m == nil {
		return nil, errors.New("unknown tz")
	}
	h, _ := strconv.Atoi(m[1]) // "+3" → 3
	minStr := m[2]
	min := 0
	if minStr != "" {
		min, _ = strconv.Atoi(minStr)
	}
	sec := h*3600 + int(math.Copysign(float64(min*60), float64(h)))
	return time.FixedZone(tz, sec), nil
}
