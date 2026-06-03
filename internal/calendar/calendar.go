package calendar

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/smerud/family-cal/internal/auth"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// HandleList returns all event instances (including expanded recurring events) within from..to.
func (s *Service) HandleList(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		http.Error(w, "from and to are required", http.StatusBadRequest)
		return
	}
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		http.Error(w, "invalid from date", http.StatusBadRequest)
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		http.Error(w, "invalid to date", http.StatusBadRequest)
		return
	}
	to = to.Add(24*time.Hour - time.Second) // inclusive end of day

	events, err := s.listExpanded(from, to)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

func (s *Service) listExpanded(from, to time.Time) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT e.id, e.title, e.description, e.start_at, e.end_at, e.all_day,
		       e.creator_id, e.color_override, e.recurrence_id,
		       r.frequency, r.interval, r.weekdays, r.until
		FROM events e
		LEFT JOIN recurrence_rules r ON r.id = e.recurrence_id
		WHERE (e.recurrence_id IS NULL AND e.start_at BETWEEN ? AND ?)
		   OR  e.recurrence_id IS NOT NULL
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exceptions, err := s.loadExceptions()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for rows.Next() {
		var (
			e             Event
			recurrenceID  sql.NullInt64
			rFreq         sql.NullString
			rInterval     sql.NullInt64
			rWeekdays     sql.NullInt64
			rUntil        sql.NullString
			description   sql.NullString
			colorOverride sql.NullString
		)
		if err := rows.Scan(
			&e.ID, &e.Title, &description, &e.StartAt, &e.EndAt, &e.AllDay,
			&e.CreatorID, &colorOverride, &recurrenceID,
			&rFreq, &rInterval, &rWeekdays, &rUntil,
		); err != nil {
			return nil, err
		}
		e.Description = description.String
		e.ColorOverride = colorOverride.String

		if !recurrenceID.Valid {
			// simple one-off event
			result = append(result, eventToMap(fmt.Sprintf("%d", e.ID), e, false, ""))
			continue
		}

		rule := RecurrenceRule{
			Frequency: RecurrenceFrequency(rFreq.String),
			Interval:  int(rInterval.Int64),
			Weekdays:  int(rWeekdays.Int64),
		}
		if rUntil.Valid && rUntil.String != "" {
			t, _ := time.Parse("2006-01-02", rUntil.String)
			rule.Until = &t
		}

		exKey := e.ID
		exMap := exceptions[exKey]

		for _, occStart := range expand(e, rule, from, to) {
			dateKey := occStart.Format("2006-01-02")
			ex, hasEx := exMap[dateKey]
			if hasEx && ex.IsDeleted {
				continue
			}
			inst := e
			inst.StartAt = occStart
			inst.EndAt = occStart.Add(e.EndAt.Sub(e.StartAt))
			if hasEx {
				if ex.OverrideTitle != "" {
					inst.Title = ex.OverrideTitle
				}
				if ex.OverrideStartAt != nil {
					inst.StartAt = *ex.OverrideStartAt
				}
				if ex.OverrideEndAt != nil {
					inst.EndAt = *ex.OverrideEndAt
				}
			}
			result = append(result, eventToMap(occurrenceID(e.ID, occStart), inst, true, dateKey))
		}
	}
	if result == nil {
		result = []map[string]interface{}{}
	}
	return result, nil
}

type exception struct {
	IsDeleted      bool
	OverrideTitle  string
	OverrideStartAt *time.Time
	OverrideEndAt   *time.Time
}

func (s *Service) loadExceptions() (map[int64]map[string]exception, error) {
	rows, err := s.db.Query(`
		SELECT event_id, original_date, is_deleted, override_start_at, override_end_at, override_title
		FROM event_exceptions
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]map[string]exception{}
	for rows.Next() {
		var (
			eventID        int64
			origDate       string
			isDeleted      int
			overrideStart  sql.NullString
			overrideEnd    sql.NullString
			overrideTitle  sql.NullString
		)
		if err := rows.Scan(&eventID, &origDate, &isDeleted, &overrideStart, &overrideEnd, &overrideTitle); err != nil {
			return nil, err
		}
		ex := exception{
			IsDeleted:     isDeleted == 1,
			OverrideTitle: overrideTitle.String,
		}
		if overrideStart.Valid {
			t, _ := time.Parse(time.RFC3339, overrideStart.String)
			ex.OverrideStartAt = &t
		}
		if overrideEnd.Valid {
			t, _ := time.Parse(time.RFC3339, overrideEnd.String)
			ex.OverrideEndAt = &t
		}
		if out[eventID] == nil {
			out[eventID] = map[string]exception{}
		}
		out[eventID][origDate] = ex
	}
	return out, nil
}

func (s *Service) HandleGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	e, err := s.getByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(eventToMap(fmt.Sprintf("%d", e.ID), *e, false, ""))
}

func (s *Service) HandleCreate(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFromContext(r.Context())
	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var recurrenceID *int64
	if req.Recurrence != nil {
		res, err := s.db.Exec(
			`INSERT INTO recurrence_rules (frequency, interval, weekdays, until) VALUES (?, ?, ?, ?)`,
			req.Recurrence.Frequency, req.Recurrence.Interval, req.Recurrence.Weekdays, req.Recurrence.Until,
		)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		rid, _ := res.LastInsertId()
		recurrenceID = &rid
	}

	res, err := s.db.Exec(
		`INSERT INTO events (title, description, start_at, end_at, all_day, creator_id, color_override, recurrence_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Title, req.Description, req.StartAt, req.EndAt, req.AllDay, u.ID, req.ColorOverride, recurrenceID,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	e, _ := s.getByID(id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(eventToMap(fmt.Sprintf("%d", e.ID), *e, false, ""))
}

func (s *Service) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	var baseID int64
	var occurrenceDate string

	if strings.Contains(idParam, "-") {
		parts := strings.SplitN(idParam, "-", 2)
		bid, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		baseID = bid
		occurrenceDate = parts[1]
	} else {
		bid, err := strconv.ParseInt(idParam, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		baseID = bid
	}

	var req struct {
		eventRequest
		Scope string `json:"scope"` // "this" | "future" | "all"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	e, err := s.getByID(baseID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	switch req.Scope {
	case "this":
		if occurrenceDate == "" {
			http.Error(w, "scope=this requires an occurrence date in the id", http.StatusBadRequest)
			return
		}
		_, err = s.db.Exec(
			`INSERT INTO event_exceptions (event_id, original_date, override_start_at, override_end_at, override_title)
			 VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(event_id, original_date) DO UPDATE SET
			   override_start_at = excluded.override_start_at,
			   override_end_at = excluded.override_end_at,
			   override_title = excluded.override_title,
			   is_deleted = 0`,
			baseID, occurrenceDate, req.StartAt, req.EndAt, req.Title,
		)
	case "future":
		if occurrenceDate == "" {
			http.Error(w, "scope=future requires an occurrence date in the id", http.StatusBadRequest)
			return
		}
		// Truncate existing rule to the day before this occurrence
		until := occurrenceDate
		if e.RecurrenceID != nil {
			s.db.Exec(`UPDATE recurrence_rules SET until = ? WHERE id = ?`, until, *e.RecurrenceID)
		}
		// Create new event + rule for future occurrences
		if req.Recurrence != nil {
			res, _ := s.db.Exec(
				`INSERT INTO recurrence_rules (frequency, interval, weekdays, until) VALUES (?, ?, ?, ?)`,
				req.Recurrence.Frequency, req.Recurrence.Interval, req.Recurrence.Weekdays, req.Recurrence.Until,
			)
			rid, _ := res.LastInsertId()
			s.db.Exec(
				`INSERT INTO events (title, description, start_at, end_at, all_day, creator_id, color_override, recurrence_id)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				req.Title, req.Description, req.StartAt, req.EndAt, req.AllDay, e.CreatorID, req.ColorOverride, rid,
			)
		}
	case "all", "":
		_, err = s.db.Exec(
			`UPDATE events SET title=?, description=?, start_at=?, end_at=?, all_day=?, color_override=? WHERE id=?`,
			req.Title, req.Description, req.StartAt, req.EndAt, req.AllDay, req.ColorOverride, baseID,
		)
	}

	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) HandleDelete(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	scope := r.URL.Query().Get("scope")

	var baseID int64
	var occurrenceDate string

	if strings.Contains(idParam, "-") {
		parts := strings.SplitN(idParam, "-", 2)
		bid, _ := strconv.ParseInt(parts[0], 10, 64)
		baseID = bid
		occurrenceDate = parts[1]
	} else {
		baseID, _ = strconv.ParseInt(idParam, 10, 64)
	}

	e, err := s.getByID(baseID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	switch scope {
	case "this":
		s.db.Exec(
			`INSERT INTO event_exceptions (event_id, original_date, is_deleted) VALUES (?, ?, 1)
			 ON CONFLICT(event_id, original_date) DO UPDATE SET is_deleted = 1`,
			baseID, occurrenceDate,
		)
	case "future":
		if e.RecurrenceID != nil {
			s.db.Exec(`UPDATE recurrence_rules SET until = ? WHERE id = ?`, occurrenceDate, *e.RecurrenceID)
		}
	default: // "all" or simple event
		if e.RecurrenceID != nil {
			s.db.Exec(`DELETE FROM recurrence_rules WHERE id = ?`, *e.RecurrenceID)
		}
		s.db.Exec(`DELETE FROM events WHERE id = ?`, baseID)
	}

	w.WriteHeader(http.StatusNoContent)
}

type recurrenceRequest struct {
	Frequency RecurrenceFrequency `json:"frequency"`
	Interval  int                 `json:"interval"`
	Weekdays  int                 `json:"weekdays"`
	Until     *string             `json:"until"`
}

type eventRequest struct {
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	StartAt       time.Time          `json:"start_at"`
	EndAt         time.Time          `json:"end_at"`
	AllDay        bool               `json:"all_day"`
	ColorOverride string             `json:"color_override"`
	Recurrence    *recurrenceRequest `json:"recurrence"`
}

func (s *Service) getByID(id int64) (*Event, error) {
	var e Event
	var recurrenceID sql.NullInt64
	var description, colorOverride sql.NullString
	err := s.db.QueryRow(`
		SELECT id, title, description, start_at, end_at, all_day, creator_id, color_override, recurrence_id, created_at
		FROM events WHERE id = ?`, id,
	).Scan(&e.ID, &e.Title, &description, &e.StartAt, &e.EndAt, &e.AllDay, &e.CreatorID, &colorOverride, &recurrenceID, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	e.Description = description.String
	e.ColorOverride = colorOverride.String
	if recurrenceID.Valid {
		e.RecurrenceID = &recurrenceID.Int64
	}
	return &e, nil
}

func eventToMap(id string, e Event, isRecurring bool, occDate string) map[string]interface{} {
	m := map[string]interface{}{
		"id":             id,
		"title":          e.Title,
		"description":    e.Description,
		"start_at":       e.StartAt,
		"end_at":         e.EndAt,
		"all_day":        e.AllDay,
		"creator_id":     e.CreatorID,
		"color_override": e.ColorOverride,
		"is_recurring":   isRecurring,
	}
	if occDate != "" {
		m["occurrence_date"] = occDate
	}
	return m
}
