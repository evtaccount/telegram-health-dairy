package config

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	DataDir       string
	TelegramToken string
}

func Load() Config {
	return Config{
		DataDir:       DataDir,
		TelegramToken: getBotToken(),
	}
}

func getBotToken() string {
	if data, err := os.ReadFile("/run/secrets/telegram_bot_token"); err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			log.Println("✅ Токен получен из Docker Secret (/run/secrets/telegram_bot_token)")
			return token
		}
	}
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if token != "" {
		log.Println("✅ Токен получен из переменной окружения (TELEGRAM_BOT_TOKEN)")
		return token
	}
	log.Fatal("❌ Токен не найден: отсутствует и Docker Secret, и переменная окружения")
	return ""
}

const (
	DataDir = "./data"
	logDir  = "./logs"
)
