package messages

import (
	"strconv"
	"telegram-health-dairy/internal/models"
	"telegram-health-dairy/internal/storage"
	"telegram-health-dairy/internal/utils"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	btnAteNow = "Поел"
	btnAteAt  = "Поел в …"
)

func SendEvening(bot *tgbotapi.BotAPI, db *storage.DB, u *models.User, dateKey string) error {
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
	m, err := bot.Send(msg)
	utils.LogFor(err)

	return db.InsertPending(&models.PendingMessage{
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
