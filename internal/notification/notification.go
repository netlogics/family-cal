package notification

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"gopkg.in/gomail.v2"
)

type Config struct {
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

type Scheduler struct {
	db   *sql.DB
	cfg  Config
}

func NewScheduler(db *sql.DB, cfg Config) *Scheduler {
	return &Scheduler{db: db, cfg: cfg}
}

// Start runs the notification scheduler in a background goroutine.
func (s *Scheduler) Start() {
	go func() {
		s.run()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			s.run()
		}
	}()
}

func (s *Scheduler) run() {
	now := time.Now()
	window := now.Add(24 * time.Hour)

	rows, err := s.db.Query(`
		SELECT e.id, e.title, e.start_at
		FROM events e
		WHERE e.start_at BETWEEN ? AND ?
		  AND e.recurrence_id IS NULL
	`, now, window)
	if err != nil {
		log.Printf("notification: query events: %v", err)
		return
	}
	defer rows.Close()

	type upcoming struct {
		id      int64
		title   string
		startAt time.Time
	}
	var events []upcoming
	for rows.Next() {
		var ev upcoming
		if err := rows.Scan(&ev.id, &ev.title, &ev.startAt); err != nil {
			continue
		}
		events = append(events, ev)
	}

	if len(events) == 0 {
		return
	}

	userRows, err := s.db.Query(`
		SELECT u.id, u.email, u.name, COALESCE(p.minutes_before, 30)
		FROM users u
		LEFT JOIN user_notification_prefs p ON p.user_id = u.id
	`)
	if err != nil {
		log.Printf("notification: query users: %v", err)
		return
	}
	defer userRows.Close()

	type userPref struct {
		id            int64
		email         string
		name          string
		minutesBefore int
	}
	var users []userPref
	for userRows.Next() {
		var u userPref
		if err := userRows.Scan(&u.id, &u.email, &u.name, &u.minutesBefore); err != nil {
			continue
		}
		users = append(users, u)
	}

	dialer := gomail.NewDialer(s.cfg.SMTPHost, s.cfg.SMTPPort, s.cfg.SMTPUser, s.cfg.SMTPPass)

	for _, ev := range events {
		for _, u := range users {
			scheduledAt := ev.startAt.Add(-time.Duration(u.minutesBefore) * time.Minute)
			if scheduledAt.After(now) {
				continue // not time yet
			}

			// Check deduplication
			var count int
			s.db.QueryRow(`
				SELECT COUNT(*) FROM notification_log
				WHERE event_id = ? AND user_id = ? AND scheduled_at = ?
			`, ev.id, u.id, scheduledAt).Scan(&count)
			if count > 0 {
				continue
			}

			if err := s.sendEmail(dialer, u.email, u.name, ev.title, ev.startAt); err != nil {
				log.Printf("notification: send to %s: %v", u.email, err)
				continue
			}

			s.db.Exec(`
				INSERT INTO notification_log (event_id, user_id, scheduled_at) VALUES (?, ?, ?)
			`, ev.id, u.id, scheduledAt)
		}
	}
}

func (s *Scheduler) sendEmail(dialer *gomail.Dialer, to, name, eventTitle string, startAt time.Time) error {
	m := gomail.NewMessage()
	m.SetHeader("From", s.cfg.SMTPFrom)
	m.SetHeader("To", to)
	m.SetHeader("Subject", fmt.Sprintf("Reminder: %s", eventTitle))
	m.SetBody("text/plain", fmt.Sprintf(
		"Hi %s,\n\nThis is a reminder that \"%s\" starts at %s.\n\nFamily Calendar",
		name, eventTitle, startAt.Format("Monday, January 2 at 3:04 PM"),
	))
	return dialer.DialAndSend(m)
}
