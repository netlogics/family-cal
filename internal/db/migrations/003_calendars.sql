CREATE TABLE calendars (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL,
  color      TEXT NOT NULL DEFAULT '#6366f1',
  created_by INTEGER NOT NULL REFERENCES users(id),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- For existing installs: seed default calendar before altering events
INSERT INTO calendars (name, color, created_by)
SELECT 'Activities', '#6366f1', id
FROM users
WHERE is_admin = 1
ORDER BY id
LIMIT 1;

-- Add calendar_id to events; DEFAULT 1 covers existing rows (Activities = id 1)
ALTER TABLE events ADD COLUMN calendar_id INTEGER NOT NULL DEFAULT 1 REFERENCES calendars(id);
