package scheduler

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/go-co-op/gocron/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"telegram-health-dairy/internal/models"
	"telegram-health-dairy/internal/storage"
)

// inline‑кнопки
var morningKB = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Жалобы", "Жалобы"),
		tgbotapi.NewInlineKeyboardButtonData("Нет жалоб", "Нет жалоб"),
	),
)
var eveningKB = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Поел", "Поел"),
		tgbotapi.NewInlineKeyboardButtonData("Поел в …", "Поел в …"),
	),
)

// Start — запускает минутный cron‑таск.
// Больше **не** импортирует пакет handlers → нет циклической зависимости.
func Start(bot *tgbotapi.BotAPI, db *storage.DB) (gocron.Scheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	_, err = s.NewJob(
		gocron.DurationJob(1*time.Minute),
		gocron.NewTask(func() {
			rows, err := db.Query(`SELECT chat_id, tz, morning_at, evening_at FROM users`)
			if err != nil {
				return
			}
			defer rows.Close()

			for rows.Next() {
				var chatID int64
				var tz, morning, evening string
				_ = rows.Scan(&chatID, &tz, &morning, &evening)

				loc, err := tzToLocation(tz)
				if err != nil {
					continue
				}

				now := time.Now().In(loc)
				day := now.Format("2006-01-02")

				// ---------- утро ----------
				if now.Format("15:04") == morning {
					key := day + "-morning"
					if !db.HasPending(chatID, key) {
						msg := tgbotapi.NewMessage(chatID, "Доброе утро! Как самочувствие?")
						msg.ReplyMarkup = morningKB
						sent, _ := bot.Send(msg)

						db.InsertPending(&models.PendingMessage{
							ChatID:    chatID,
							DateKey:   key,
							Type:      "morning",
							MsgID:     sent.MessageID,
							CreatedAt: time.Now().Unix(),
						})
						db.SetSessionState(chatID, models.StateWaitingMorning)
					}
				}

				// ---------- вечер ----------
				if now.Format("15:04") == evening {
					key := day + "-evening"
					if !db.HasPending(chatID, key) {
						hrsLeft := 23 - now.Hour()
						txt := "Пора ужинать! До конца дня осталось " + strconv.Itoa(hrsLeft) + " ч."
						msg := tgbotapi.NewMessage(chatID, txt)
						msg.ReplyMarkup = eveningKB
						sent, _ := bot.Send(msg)

						db.InsertPending(&models.PendingMessage{
							ChatID:    chatID,
							DateKey:   key,
							Type:      "evening",
							MsgID:     sent.MessageID,
							CreatedAt: time.Now().Unix(),
						})
						db.SetSessionState(chatID, models.StateWaitingEvening)
					}
				}
			}
		}),
	)
	if err != nil {
		return nil, err
	}

	s.Start()
	return s, nil
}

var offRx = regexp.MustCompile(`^(?i)(gmt|utc)?([+-]\d{1,2})(?::?(\d{2}))?$`)

func tzToLocation(tz string) (*time.Location, error) {
	// пробуем IANA
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc, nil
	}
	// offset?
	m := offRx.FindStringSubmatch(tz)
	if m == nil {
		return nil, errors.New("unknown tz")
	}
	h, _ := strconv.Atoi(m[2])
	mnt := 0
	if m[3] != "" {
		mnt, _ = strconv.Atoi(m[3])
	}
	offset := h*3600 + int(math.Copysign(float64(mnt*60), float64(h)))
	return time.FixedZone(tz, offset), nil
}
