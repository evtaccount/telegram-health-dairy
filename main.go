package main

import (
	"os"

	"telegram-health-diary/internal/handlers"
	"telegram-health-diary/internal/scheduler"
	"telegram-health-diary/internal/storage"
	"telegram-health-diary/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load() // TELEGRAM_BOT_TOKEN etc.

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	utils.Must(err)

	db, err := storage.New("bot.db")
	utils.Must(err)

	h := &handlers.Handler{Bot: bot, DB: db}
	_, err = scheduler.Start(h, db)
	utils.Must(err)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	for upd := range updates {
		if upd.Message != nil {
			if upd.Message.IsCommand() && upd.Message.Command() == "start" {
				h.HandleStart(upd.Message)
				continue
			}
			h.HandleText(upd.Message)
		}
		if upd.CallbackQuery != nil {
			h.HandleCallback(upd.CallbackQuery)
		}
	}
}
