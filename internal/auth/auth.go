package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const cookieName = "session"

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
)

type contextKey struct{}

type Service struct {
	db         *sql.DB
	jwtSecret  []byte
	setupToken string
}

func NewService(db *sql.DB, jwtSecret string) *Service {
	return &Service{db: db, jwtSecret: []byte(jwtSecret)}
}

// SetupToken returns a one-time token for initial admin creation.
// It is generated once and stored in memory for the lifetime of the process.
func (s *Service) SetupToken() (string, error) {
	if s.setupToken != "" {
		return s.setupToken, nil
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return "", err
	}
	if count > 0 {
		return "", nil // setup already done
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	s.setupToken = hex.EncodeToString(b)
	return s.setupToken, nil
}

func (s *Service) CreateAdmin(token, email, name, password string) (*User, error) {
	if s.setupToken == "" || token != s.setupToken {
		return nil, errors.New("invalid setup token")
	}
	u, err := s.CreateUser(email, name, password, "#6366f1", true)
	if err != nil {
		return nil, err
	}
	s.setupToken = "" // invalidate after use
	return u, nil
}

func (s *Service) CreateUser(email, name, password, color string, isAdmin bool) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	res, err := s.db.Exec(
		`INSERT INTO users (email, name, password_hash, color, is_admin) VALUES (?, ?, ?, ?, ?)`,
		email, name, string(hash), color, isAdmin,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetByID(id)
}

func (s *Service) Login(email, password string) (*User, string, error) {
	u, err := s.GetByEmail(email)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}
	token, err := s.issueToken(u.ID)
	if err != nil {
		return nil, "", err
	}
	return u, token, nil
}

func (s *Service) issueToken(userID int64) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func (s *Service) SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 60 * 60,
	})
}

func (s *Service) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// Middleware validates the JWT cookie and injects the user into context.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		token, err := jwt.Parse(cookie.Value, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return s.jwtSecret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		userID := int64(claims["sub"].(float64))
		u, err := s.GetByID(userID)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), contextKey{}, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminOnly middleware requires the authenticated user to be an admin.
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil || !u.IsAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(contextKey{}).(*User)
	return u
}

func (s *Service) GetByID(id int64) (*User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, email, name, password_hash, color, is_admin, dark_mode, created_at FROM users WHERE id = ?`, id,
	))
}

func (s *Service) GetByEmail(email string) (*User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, email, name, password_hash, color, is_admin, dark_mode, created_at FROM users WHERE email = ?`, email,
	))
}

func (s *Service) scanUser(row *sql.Row) (*User, error) {
	var u User
	var isAdmin, darkMode int
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Color, &isAdmin, &darkMode, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	u.IsAdmin = isAdmin == 1
	u.DarkMode = darkMode == 1
	return &u, nil
}
