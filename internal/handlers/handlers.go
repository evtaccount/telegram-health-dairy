package handlers

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"telegram-health-diary/internal/models"
	"telegram-health-diary/internal/storage"
	"telegram-health-diary/internal/utils"

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
)

type Handler struct {
	Bot *tgbotapi.BotAPI
	DB  *storage.DB
}

// ---------------- /start --------------------
func (h *Handler) HandleStart(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	u, _ := h.DB.GetUser(chatID)
	if u == nil {
		// create with defaults
		_ = h.DB.UpsertUser(&models.User{
			ChatID:    chatID,
			TZ:        "Europe/Moscow",
			MorningAt: "10:00",
			EveningAt: "18:00",
		})
	}
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(menuStats),
			tgbotapi.NewKeyboardButton(menuMorning),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(menuEvening),
			tgbotapi.NewKeyboardButton(menuTZ),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(menuClear),
		),
	)

	reply := tgbotapi.NewMessage(chatID, "Главное меню")
	reply.ReplyMarkup = kb
	_, _ = h.Bot.Send(reply)
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

// ------------- callbacks ------------------
func (h *Handler) HandleCallback(cq *tgbotapi.CallbackQuery) {
	data := cq.Data
	chatID := cq.Message.Chat.ID
	dateKey := extractDateKey(cq.Message.Time(), data) // helper below

	switch data {
	case btnComplaints, btnNoComplaints:
		rec, _ := h.DB.GetDayRecord(chatID, dateKey[:10])
		if rec != nil && rec.Complaints != "" {
			// already answered -> ask confirmation
			kb := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(btnChange, "chg:"+data),
					tgbotapi.NewInlineKeyboardButtonData(btnCancel, btnCancel),
				),
			)
			_, _ = h.Bot.Request(tgbotapi.NewEditMessageReplyMarkup(chatID, cq.Message.MessageID, kb))
			return
		}
		if data == btnComplaints {
			_ = h.DB.SetUserState(chatID, "wait_complaints:"+dateKey)
			_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Опиши жалобы текстом"))
		} else {
			_ = h.DB.UpsertDayRecord(chatID, dateKey[:10], "")
			_ = h.DB.DeletePending(chatID, dateKey)
			_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Хорошего дня!"))
		}

	case btnAteNow:
		now := time.Now()
		_ = h.DB.SetDinner(chatID, dateKey[:10], now)
		_ = h.DB.DeletePending(chatID, dateKey)
		_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Приятного вечера!"))

	case btnAteAt:
		_ = h.DB.SetUserState(chatID, "wait_dinner_time:"+dateKey)
		_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Во сколько поужинал? (HH:MM)"))

	case btnCancel:
		h.Bot.Request(tgbotapi.NewCallback(cq.ID, "Отменено"))
	}

	// always answer callback to remove 'loading...'
	h.Bot.Request(tgbotapi.NewCallback(cq.ID, ""))
}

func extractDateKey(t time.Time, typ string) string {
	d := t.UTC().Format("2006-01-02")
	if strings.HasPrefix(typ, "Жалоб") || strings.HasPrefix(typ, "chg:") {
		return d + "-morning"
	}
	return d + "-evening"
}

// ------------- text -----------------------
func (h *Handler) HandleText(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	state, _ := h.DB.GetUserState(chatID)
	if state == "" {
		return
	}

	switch {
	case strings.HasPrefix(state, "wait_complaints:"):
		dateKey := strings.TrimPrefix(state, "wait_complaints:")
		_ = h.DB.UpsertDayRecord(chatID, dateKey[:10], msg.Text)
		_ = h.DB.DeletePending(chatID, dateKey)
		_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Жалобы записаны."))
	case strings.HasPrefix(state, "wait_dinner_time:"):
		if !timeRx.MatchString(msg.Text) {
			_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Формат HH:MM"))
			return
		}
		tparts := strings.Split(msg.Text, ":")
		hStr, _ := strconv.Atoi(tparts[0])
		mStr, _ := strconv.Atoi(tparts[1])
		now := time.Now()
		dt := time.Date(now.Year(), now.Month(), now.Day(), hStr, mStr, 0, 0, time.UTC)
		dateKey := strings.TrimPrefix(state, "wait_dinner_time:")
		_ = h.DB.SetDinner(chatID, dateKey[:10], dt)
		_ = h.DB.DeletePending(chatID, dateKey)
		_, _ = h.Bot.Send(tgbotapi.NewMessage(chatID, "Записал, спасибо!"))
	}
	_ = h.DB.SetUserState(chatID, "")
}
