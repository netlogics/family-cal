CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  color TEXT NOT NULL DEFAULT '#3b82f6',
  is_admin INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE recurrence_rules (
  id INTEGER PRIMARY KEY,
  frequency TEXT NOT NULL,
  interval INTEGER NOT NULL DEFAULT 1,
  weekdays INTEGER NOT NULL DEFAULT 0,
  until DATE
);

CREATE TABLE events (
  id INTEGER PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT,
  start_at DATETIME NOT NULL,
  end_at DATETIME NOT NULL,
  all_day INTEGER NOT NULL DEFAULT 0,
  creator_id INTEGER NOT NULL REFERENCES users(id),
  color_override TEXT,
  recurrence_id INTEGER REFERENCES recurrence_rules(id),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE event_exceptions (
  id INTEGER PRIMARY KEY,
  event_id INTEGER NOT NULL REFERENCES events(id),
  original_date DATE NOT NULL,
  is_deleted INTEGER NOT NULL DEFAULT 0,
  override_start_at DATETIME,
  override_end_at DATETIME,
  override_title TEXT,
  UNIQUE(event_id, original_date)
);

CREATE TABLE user_notification_prefs (
  user_id INTEGER PRIMARY KEY REFERENCES users(id),
  minutes_before INTEGER NOT NULL DEFAULT 30
);

CREATE TABLE notification_log (
  id INTEGER PRIMARY KEY,
  event_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  scheduled_at DATETIME NOT NULL,
  sent_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

