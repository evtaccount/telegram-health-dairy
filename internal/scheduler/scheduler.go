package scheduler

import (
	"log"
	"time"

	"telegram-health-diary/internal/handlers"
	"telegram-health-diary/internal/storage"
	"telegram-health-diary/internal/utils"

	"github.com/go-co-op/gocron/v2"
)

func Start(h *handlers.Handler, db *storage.DB) (gocron.Scheduler, error) {
	// Создаём новый планировщик
	s, err := gocron.NewScheduler()
	utils.Must(err)

	// Регистрируем задачу с периодом 1 минута
	_, err = s.NewJob(
		gocron.DurationJob(1*time.Minute),
		gocron.NewTask(func() {
			users, err := db.ListUsers()
			if err != nil {
				log.Println("Ошибка чтения пользователей:", err)
				return
			}

			nowUTC := time.Now().UTC()

			for _, u := range users {
				loc, err := time.LoadLocation(u.TZ)
				if err != nil {
					log.Printf("Некорректный часовой пояс %s для %d\n", u.TZ, u.ChatID)
					continue
				}

				now := nowUTC.In(loc)
				day := now.Format("2006-01-02")

				// Утро
				if now.Format("15:04") == u.MorningAt {
					dateKey := day + "-morning"
					if !db.HasPendingOrAnswered(u.ChatID, dateKey) {
						if err := h.SendMorning(&u, dateKey); err != nil {
							log.Println("Ошибка отправки утреннего сообщения:", err)
						}
					}
				}

				// Вечер
				if now.Format("15:04") == u.EveningAt {
					dateKey := day + "-evening"
					if !db.HasPendingOrAnswered(u.ChatID, dateKey) {
						if err := h.SendEvening(&u, dateKey); err != nil {
							log.Println("Ошибка отправки вечернего сообщения:", err)
						}
					}
				}
			}
		}),
	)
	if err != nil {
		return nil, err
	}

	// Запускаем планировщик
	s.Start()
	return s, nil
}
