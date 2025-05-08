package storage

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"telegram-health-dairy/internal/models"
)

//go:embed schema.sql
var ddl embed.FS

type DB struct{ *sql.DB }

func (d *DB) DropAll() error {
	d.Close()
	return os.Remove("bot.db")
}

// ClearData полностью очищает все данные по пользователю
func (d *DB) ClearData(chatID int64) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tables := []string{
		"day_records",
		"pending_messages",
		"user_states",
		"sessions",
		"users",
	}
	for _, tbl := range tables {
		if _, err := tx.Exec(
			fmt.Sprintf("DELETE FROM %s WHERE chat_id = ?", tbl),
			chatID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if err = migrate(db); err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

func migrate(db *sql.DB) error {
	b, err := ddl.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	_, err = db.Exec(string(b))
	return err
}

// ---------- users -----------------------------------------------------------

func (d *DB) UpsertUser(u *models.User) error {
	_, err := d.Exec(`
        INSERT INTO users (chat_id, tz, morning_at, evening_at, created_at)
        VALUES (?,?,?,?,?)
        ON CONFLICT(chat_id) DO UPDATE SET tz=excluded.tz,
            morning_at=excluded.morning_at,
            evening_at=excluded.evening_at
    `, u.ChatID, u.TZ, u.MorningAt, u.EveningAt, time.Now().Unix())
	return err
}

func (d *DB) GetUser(chatID int64) (*models.User, error) {
	var u models.User

	err := d.QueryRow(`
        SELECT id, chat_id, tz, morning_at, evening_at, created_at
        FROM users WHERE chat_id=?`, chatID,
	).Scan(&u.ID, &u.ChatID, &u.TZ, &u.MorningAt, &u.EveningAt, &u.CreatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return &u, err
}

func (d *DB) ListUsers() ([]models.User, error) {
	rows, err := d.Query(`SELECT id, chat_id, tz, morning_at, evening_at, created_at FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.ChatID, &u.TZ, &u.MorningAt, &u.EveningAt, &u.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, u)
	}
	return res, nil
}

// ---------- user state (fsm) ------------------------------------------------

func (d *DB) SetUserState(chatID int64, state string) error {
	_, err := d.Exec(`
        INSERT INTO user_states(chat_id, state) VALUES (?,?)
        ON CONFLICT(chat_id) DO UPDATE SET state=excluded.state`, chatID, state)
	return err
}

func (d *DB) GetUserState(chatID int64) (string, error) {
	var st string
	err := d.QueryRow(`SELECT state FROM user_states WHERE chat_id=?`, chatID).Scan(&st)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return st, err
}

// ---------- day records -----------------------------------------------------

func (d *DB) UpsertDayRecord(chatID int64, day, complaints string) error {
	_, err := d.Exec(`
        INSERT INTO day_records(chat_id, day, complaints) VALUES (?,?,?)
        ON CONFLICT(chat_id,day) DO UPDATE SET complaints=excluded.complaints
    `, chatID, day, complaints)
	return err
}

func (d *DB) SetDinner(chatID int64, day string, t time.Time) error {
	_, err := d.Exec(`
        INSERT INTO day_records(chat_id, day, dinner_at) VALUES (?,?,?)
        ON CONFLICT(chat_id,day) DO UPDATE SET dinner_at=excluded.dinner_at
    `, chatID, day, t.Unix())
	return err
}

func (d *DB) GetDayRecord(chatID int64, day string) (*models.DayRecord, error) {
	var rec models.DayRecord
	var dinnerTs sql.NullInt64
	err := d.QueryRow(`
        SELECT id, chat_id, day, complaints, dinner_at
        FROM day_records WHERE chat_id=? AND day=?`, chatID, day,
	).Scan(&rec.ID, &rec.ChatID, &rec.Day, &rec.Complaints, &dinnerTs)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if dinnerTs.Valid {
		t := time.Unix(dinnerTs.Int64, 0)
		rec.DinnerAt = &t
	}
	return &rec, nil
}

// ---------- pending ---------------------------------------------------------

// InsertPending: теперь инициализируем reminded_at = 0
func (d *DB) InsertPending(p *models.PendingMessage) error {
	now := time.Now().Unix()

	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	if p.RemindedAt == 0 {
		p.RemindedAt = now // ← СРАЗУ = время отправки вопроса
	}

	_, err := d.Exec(`
        INSERT OR REPLACE INTO pending_messages
          (chat_id, date_key, type, msg_id, created_at, reminded_at)
        VALUES (?,?,?,?,?,?)
    `, p.ChatID, p.DateKey, p.Type, p.MsgID, p.CreatedAt, p.RemindedAt)
	return err
}

// ListPendingForReminder возвращает pending’и, которым пора напомнить
// (прошло ≥ 1200 сек и пользователь ещё не ответил)
func (d *DB) ListPendingForReminder(chatID int64) ([]models.PendingMessage, error) {
	rows, err := d.Query(`
        SELECT id, chat_id, date_key, type, msg_id, created_at, reminded_at
        FROM pending_messages
        WHERE chat_id = ?
          AND reminded_at < strftime('%s','now') - 1200
    `, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []models.PendingMessage
	for rows.Next() {
		var p models.PendingMessage
		if err := rows.Scan(
			&p.ID, &p.ChatID, &p.DateKey, &p.Type, &p.MsgID,
			&p.CreatedAt, &p.RemindedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, p)
	}
	return res, nil
}

// TouchReminder помечает, что напоминание отправлено (обновляет reminded_at)
func (d *DB) TouchReminder(id int64) {
	_, _ = d.Exec(`UPDATE pending_messages
	               SET reminded_at = strftime('%s','now')
	               WHERE id = ?`, id)
}

func (d *DB) DeletePending(chatID int64, dateKey string) error {
	_, err := d.Exec(`DELETE FROM pending_messages WHERE chat_id=? AND date_key=?`, chatID, dateKey)
	return err
}

func (d *DB) HasPending(chatID int64, dateKey string) bool {
	var c int
	_ = d.QueryRow(`SELECT 1 FROM pending_messages WHERE chat_id=? AND date_key=?`, chatID, dateKey).Scan(&c)
	return c == 1
}

// answered? reuse day_records for determination
func (d *DB) HasAnswered(chatID int64, dateKey string) bool {
	parts := strings.Split(dateKey, "-")
	if len(parts) < 4 {
		return false
	}
	day := strings.Join(parts[:3], "-")
	t := parts[3]
	rec, _ := d.GetDayRecord(chatID, day)
	if rec == nil {
		return false
	}
	if t == "morning" {
		return rec.Complaints != ""
	}
	if t == "evening" {
		return rec.DinnerAt != nil
	}
	return false
}

func (d *DB) HasPendingOrAnswered(chatID int64, dateKey string) bool {
	return d.HasPending(chatID, dateKey) || d.HasAnswered(chatID, dateKey)
}

// Session state handling
func (d *DB) SetSessionState(chatID int64, state models.State) error {
	_, err := d.Exec(`
		INSERT INTO sessions (chat_id, state)
		VALUES (?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET state=excluded.state
	`, chatID, state)
	return err
}

func (d *DB) GetSessionState(chatID int64) (models.State, error) {
	var state string
	err := d.QueryRow(`SELECT state FROM sessions WHERE chat_id = ?`, chatID).Scan(&state)
	if err == sql.ErrNoRows {
		return models.StateNotStarted, nil
	}
	if err != nil {
		return "", err
	}
	return models.State(state), nil
}
