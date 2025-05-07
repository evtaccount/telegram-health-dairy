package main

import (
	"telegram-health-dairy/internal/bot"
	"telegram-health-dairy/internal/config"
	"telegram-health-dairy/internal/handlers"
	"telegram-health-dairy/internal/storage"
	"telegram-health-dairy/internal/utils"
)

func main() {
	cfg := config.Load()

	bot, err := bot.New(cfg.TelegramToken)
	utils.Must(err)

	db, err := storage.New(cfg.DBName)
	utils.Must(err)

	handlers.Register(bot.API, db)

	bot.Run()
}
