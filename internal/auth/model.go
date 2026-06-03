package auth

import "time"

type User struct {
	ID           int64
	Email        string
	Name         string
	PasswordHash string
	Color        string
	IsAdmin      bool
	DarkMode     bool
	CreatedAt    time.Time
}
