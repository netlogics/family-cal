package user

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/smerud/family-cal/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) HandleList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT id, email, name, color, is_admin, created_at FROM users ORDER BY name`,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var (
			id      int64
			email   string
			name    string
			color   string
			isAdmin int
			created string
		)
		if err := rows.Scan(&id, &email, &name, &color, &isAdmin, &created); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		users = append(users, map[string]interface{}{
			"id":       id,
			"email":    email,
			"name":     name,
			"color":    color,
			"is_admin": isAdmin == 1,
		})
	}
	if users == nil {
		users = []map[string]interface{}{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (s *Service) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
		Color    string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Color == "" {
		req.Color = "#3b82f6"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	res, err := s.db.Exec(
		`INSERT INTO users (email, name, password_hash, color) VALUES (?, ?, ?, ?)`,
		req.Email, req.Name, string(hash), req.Color,
	)
	if err != nil {
		http.Error(w, "email already in use", http.StatusConflict)
		return
	}
	id, _ := res.LastInsertId()

	// create default notification pref
	s.db.Exec(`INSERT INTO user_notification_prefs (user_id) VALUES (?)`, id)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":    id,
		"email": req.Email,
		"name":  req.Name,
		"color": req.Color,
	})
}

func (s *Service) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	caller := auth.UserFromContext(r.Context())
	if !caller.IsAdmin && caller.ID != targetID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Name            string `json:"name"`
		Color           string `json:"color"`
		MinutesBefore   *int   `json:"minutes_before"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.Name != "" || req.Color != "" {
		s.db.Exec(
			`UPDATE users SET name = COALESCE(NULLIF(?, ''), name), color = COALESCE(NULLIF(?, ''), color) WHERE id = ?`,
			req.Name, req.Color, targetID,
		)
	}
	if req.MinutesBefore != nil {
		s.db.Exec(
			`INSERT INTO user_notification_prefs (user_id, minutes_before) VALUES (?, ?)
			 ON CONFLICT(user_id) DO UPDATE SET minutes_before = excluded.minutes_before`,
			targetID, *req.MinutesBefore,
		)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) HandleDelete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	targetID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	caller := auth.UserFromContext(r.Context())
	if caller.ID == targetID {
		http.Error(w, "cannot delete yourself", http.StatusBadRequest)
		return
	}
	s.db.Exec(`DELETE FROM users WHERE id = ?`, targetID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) HandleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req struct {
		DarkMode bool `json:"dark_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	darkModeInt := 0
	if req.DarkMode {
		darkModeInt = 1
	}
	if _, err := s.db.Exec(`UPDATE users SET dark_mode = ? WHERE id = ?`, darkModeInt, u.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var hash string
	if err := s.db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, u.ID).Scan(&hash); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.CurrentPassword)); err != nil {
		http.Error(w, "incorrect current password", http.StatusUnauthorized)
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if _, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(newHash), u.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
