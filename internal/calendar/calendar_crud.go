package calendar

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/smerud/family-cal/internal/auth"
)

func parseID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func (s *Service) HandleListCalendars(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, name, color, created_by FROM calendars ORDER BY id`)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var cals []Calendar
	for rows.Next() {
		var c Calendar
		if err := rows.Scan(&c.ID, &c.Name, &c.Color, &c.CreatedBy); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		cals = append(cals, c)
	}
	if cals == nil {
		cals = []Calendar{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cals)
}

func (s *Service) HandleCreateCalendar(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}
	res, err := s.db.Exec(
		`INSERT INTO calendars (name, color, created_by) VALUES (?, ?, ?)`,
		req.Name, req.Color, u.ID,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	c := Calendar{ID: id, Name: req.Name, Color: req.Color, CreatedBy: u.ID}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

func (s *Service) HandleUpdateCalendar(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	res, err := s.db.Exec(
		`UPDATE calendars SET
		   name  = COALESCE(NULLIF(?, ''), name),
		   color = COALESCE(NULLIF(?, ''), color)
		 WHERE id = ?`,
		req.Name, req.Color, id,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) HandleDeleteCalendar(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if id == 1 {
		http.Error(w, "cannot delete the default calendar", http.StatusBadRequest)
		return
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE calendar_id = ?`, id).Scan(&count); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		http.Error(w, "calendar has events", http.StatusConflict)
		return
	}
	if _, err := s.db.Exec(`DELETE FROM calendars WHERE id = ?`, id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
