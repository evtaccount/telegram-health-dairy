PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users(
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  chat_id     INTEGER UNIQUE,
  tz          TEXT    NOT NULL DEFAULT 'Europe/Moscow',
  morning_at  TEXT    NOT NULL DEFAULT '10:00',
  evening_at  TEXT    NOT NULL DEFAULT '18:00',
  created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS day_records(
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  chat_id     INTEGER NOT NULL REFERENCES users(chat_id) ON DELETE CASCADE,
  day         TEXT    NOT NULL,
  complaints  TEXT,
  dinner_at   INTEGER,
  UNIQUE(chat_id, day)
);

CREATE TABLE IF NOT EXISTS pending_messages(
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  chat_id     INTEGER NOT NULL,
  date_key    TEXT    NOT NULL,
  type        TEXT    NOT NULL,
  msg_id      INTEGER NOT NULL,
  created_at  INTEGER NOT NULL,
  reminded_at INTEGER NOT NULL DEFAULT 0,
  UNIQUE(chat_id, date_key)
);

CREATE TABLE IF NOT EXISTS user_states(
  chat_id INTEGER PRIMARY KEY,
  state   TEXT
);

CREATE TABLE IF NOT EXISTS sessions(
  chat_id INTEGER PRIMARY KEY,
  state   TEXT NOT NULL
);