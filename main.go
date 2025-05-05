package main

import (
	"log"
	"os"
	"telegram-health-dairy/internal/config"
	"telegram-health-dairy/internal/handlers"
	"telegram-health-dairy/internal/scheduler"
	"telegram-health-dairy/internal/storage"
	"telegram-health-dairy/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	cfg := config.Load()
	log.Println("TELEGRAM_BOT_TOKEN=", os.Getenv("TELEGRAM_BOT_TOKEN"))

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
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
