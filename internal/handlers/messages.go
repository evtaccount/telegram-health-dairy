package handlers

import (
	"regexp"
	"strconv"
	"time"

	"telegram-health-dairy/internal/models"
	"telegram-health-dairy/internal/storage"
	"telegram-health-dairy/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var timeRx = regexp.MustCompile(`^\d{1,2}:\d{2}$`)

const (
	btnComplaints   = "Жалобы"
	btnNoComplaints = "Нет жалоб"
	btnAteNow       = "Поел"
	btnAteAt        = "Поел в …"

	btnChange = "Изменить"
	btnCancel = "Отмена"

	menuStats   = "Показать статистику"
	menuMorning = "Задать время утреннего сообщения"
	menuEvening = "Задать время вечернего сообщения"
	menuTZ      = "Сменить часовой пояс"
	menuClear   = "Очистить данные"

	actionMorning = "Самочувствие утром"
	actionEvening = "Ужин в ..."
)

type Handler struct {
	Bot *tgbotapi.BotAPI
	DB  *storage.DB
}

func (h *Handler) HandleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	if msg.IsCommand() {
		h.HandleCommand(chatID, msg.Command())
	} else {
		h.HandleText(msg)
	}
}

// --- morning / evening messages ----------
func (h *Handler) SendMorning(u *models.User, dateKey string) error {
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(btnComplaints, btnComplaints),
			tgbotapi.NewInlineKeyboardButtonData(btnNoComplaints, btnNoComplaints),
		),
	)

	msg := tgbotapi.NewMessage(u.ChatID, "Доброе утро! Как самочувствие?")
	msg.ReplyMarkup = kb
	m, err := h.Bot.Send(msg)
	utils.Must(err)

	return h.DB.InsertPending(&models.PendingMessage{
		ChatID:    u.ChatID,
		DateKey:   dateKey,
		Type:      "morning",
		MsgID:     m.MessageID,
		CreatedAt: time.Now().Unix(),
	})
}

func (h *Handler) SendEvening(u *models.User, dateKey string) error {
	hrs := hoursLeft(u)
	txt := "Пора ужинать! До конца дня осталось " + strconv.Itoa(hrs) + " ч."
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(btnAteNow, btnAteNow),
			tgbotapi.NewInlineKeyboardButtonData(btnAteAt, btnAteAt),
		),
	)

	msg := tgbotapi.NewMessage(u.ChatID, txt)
	msg.ReplyMarkup = kb
	m, err := h.Bot.Send(msg)
	utils.Must(err)

	return h.DB.InsertPending(&models.PendingMessage{
		ChatID:    u.ChatID,
		DateKey:   dateKey,
		Type:      "evening",
		MsgID:     m.MessageID,
		CreatedAt: time.Now().Unix(),
	})
}

func hoursLeft(u *models.User) int {
	loc, _ := time.LoadLocation(u.TZ)
	now := time.Now().In(loc)
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 0, 0, 0, loc)
	return int(end.Sub(now).Hours())
}
