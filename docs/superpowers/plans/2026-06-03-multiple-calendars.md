# Multiple Calendars Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow admins to create named, shared calendars; all users see all calendars and can toggle them on/off in a collapsible sidebar; events belong to exactly one calendar.

**Architecture:** New `calendars` DB table with `calendar_id` FK on `events`. Calendar CRUD lives in a new `internal/calendar/calendar_crud.go` file alongside the existing event code. The frontend gains a collapsible sidebar (Alpine `sidebarOpen`) with per-calendar visibility checkboxes; filtering is purely client-side. A default "Activities" calendar is created both by the DB migration (for existing installs) and by `CreateAdmin` (for fresh installs).

**Tech Stack:** Go (chi router), SQLite, Alpine.js, Tailwind v4

---

## File Map

| Action | Path | Purpose |
|--------|------|---------|
| Create | `internal/db/migrations/003_calendars.sql` | Schema + default data migration |
| Modify | `internal/auth/auth.go` | `CreateAdmin` inserts Activities calendar |
| Modify | `internal/calendar/model.go` | Add `Calendar` struct, `CalendarID` to `Event` |
| Create | `internal/calendar/calendar_crud.go` | Calendar CRUD HTTP handlers |
| Modify | `internal/calendar/calendar.go` | Add `calendar_id` to all event queries/maps |
| Modify | `internal/api/router.go` | Register `/api/calendars` routes |
| Modify | `cmd/server/web/index.html` | Sidebar, event form picker, Alpine state |

---

### Task 1: DB migration

**Files:**
- Create: `internal/db/migrations/003_calendars.sql`

- [ ] **Step 1: Create the migration file**

`internal/db/migrations/003_calendars.sql`:
```sql
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
```

- [ ] **Step 2: Verify migration applies cleanly**

```bash
rm -f family-cal.db
go build -o family-cal ./cmd/server && ./family-cal &
sleep 1 && kill %1
```

Expected: server starts without `migration:` error in output. Fresh DB has no users so INSERT selects nothing — that is correct; CreateAdmin will insert the calendar later.

- [ ] **Step 3: Commit**

```bash
git add internal/db/migrations/003_calendars.sql
git commit -m "feat: add calendars table and calendar_id to events"
```

---

### Task 2: Update auth — CreateAdmin seeds Activities calendar

**Files:**
- Modify: `internal/auth/auth.go`

- [ ] **Step 1: Add calendar insert after user creation in CreateAdmin**

In `internal/auth/auth.go`, replace `CreateAdmin`:

```go
func (s *Service) CreateAdmin(token, email, name, password string) (*User, error) {
	if s.setupToken == "" || token != s.setupToken {
		return nil, errors.New("invalid setup token")
	}
	u, err := s.CreateUser(email, name, password, "#6366f1", true)
	if err != nil {
		return nil, err
	}
	s.db.Exec(`INSERT INTO calendars (name, color, created_by) VALUES ('Activities', '#6366f1', ?)`, u.ID)
	s.setupToken = "" // invalidate after use
	return u, nil
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

Expected: no output (success).

- [ ] **Step 3: Smoke-test fresh install**

```bash
rm -f family-cal.db && ./family-cal &
# copy the setup token from stdout, then:
curl -s -X POST http://localhost:8080/api/auth/setup \
  -H "Content-Type: application/json" \
  -d '{"token":"<TOKEN>","email":"a@b.com","name":"Admin","password":"secret"}' | jq .
kill %1
```

Expected: user JSON returned; running `sqlite3 family-cal.db "SELECT * FROM calendars;"` shows one row: `1|Activities|#6366f1|1|...`

- [ ] **Step 4: Commit**

```bash
git add internal/auth/auth.go
git commit -m "feat: seed Activities calendar on first-run setup"
```

---

### Task 3: Calendar model and CRUD handlers

**Files:**
- Modify: `internal/calendar/model.go` — add `Calendar` struct and `CalendarID` to `Event`
- Create: `internal/calendar/calendar_crud.go` — CRUD handlers

- [ ] **Step 1: Add Calendar struct and CalendarID to Event**

`internal/calendar/model.go` — add `CalendarID int64` to `Event` and append the new struct:

```go
type Event struct {
	ID            int64
	Title         string
	Description   string
	StartAt       time.Time
	EndAt         time.Time
	AllDay        bool
	CreatorID     int64
	CalendarID    int64
	ColorOverride string
	RecurrenceID  *int64
	Recurrence    *RecurrenceRule
	CreatedAt     time.Time
}

type Calendar struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	CreatedBy int64  `json:"created_by"`
}
```

(`RecurrenceRule`, `EventInstance`, constants, and `weekdayBit` are unchanged.)

- [ ] **Step 2: Create calendar_crud.go**

`internal/calendar/calendar_crud.go`:

```go
package calendar

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/smerud/family-cal/internal/auth"
)

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
	id, err := parseID(chi.URLParam(r, "id"))
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
	_, err = s.db.Exec(
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
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) HandleDeleteCalendar(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE calendar_id = ?`, id).Scan(&count)
	if count > 0 {
		http.Error(w, "calendar has events", http.StatusConflict)
		return
	}
	s.db.Exec(`DELETE FROM calendars WHERE id = ?`, id)
	w.WriteHeader(http.StatusNoContent)
}

```

Import block for `calendar_crud.go`:
```go
import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/smerud/family-cal/internal/auth"
)
```

In each handler, parse the URL param with:
```go
id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
if err != nil {
    http.Error(w, "invalid id", http.StatusBadRequest)
    return
}
```

- [ ] **Step 3: Build to check for compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add internal/calendar/model.go internal/calendar/calendar_crud.go
git commit -m "feat: add Calendar model and CRUD handlers"
```

---

### Task 4: Update event queries to include calendar_id

**Files:**
- Modify: `internal/calendar/calendar.go`

- [ ] **Step 1: Add CalendarID to eventRequest and eventToMap**

In `internal/calendar/calendar.go`:

Replace `eventRequest` struct:
```go
type eventRequest struct {
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	StartAt       time.Time          `json:"start_at"`
	EndAt         time.Time          `json:"end_at"`
	AllDay        bool               `json:"all_day"`
	CalendarID    int64              `json:"calendar_id"`
	ColorOverride string             `json:"color_override"`
	Recurrence    *recurrenceRequest `json:"recurrence"`
}
```

Replace `eventToMap`:
```go
func eventToMap(id string, e Event, isRecurring bool, occDate string) map[string]interface{} {
	m := map[string]interface{}{
		"id":             id,
		"title":          e.Title,
		"description":    e.Description,
		"start_at":       e.StartAt,
		"end_at":         e.EndAt,
		"all_day":        e.AllDay,
		"creator_id":     e.CreatorID,
		"calendar_id":    e.CalendarID,
		"color_override": e.ColorOverride,
		"is_recurring":   isRecurring,
	}
	if occDate != "" {
		m["occurrence_date"] = occDate
	}
	return m
}
```

- [ ] **Step 2: Update getByID to scan calendar_id**

Replace `getByID`:
```go
func (s *Service) getByID(id int64) (*Event, error) {
	var e Event
	var recurrenceID sql.NullInt64
	var description, colorOverride sql.NullString
	err := s.db.QueryRow(`
		SELECT id, title, description, start_at, end_at, all_day,
		       creator_id, calendar_id, color_override, recurrence_id, created_at
		FROM events WHERE id = ?`, id,
	).Scan(&e.ID, &e.Title, &description, &e.StartAt, &e.EndAt, &e.AllDay,
		&e.CreatorID, &e.CalendarID, &colorOverride, &recurrenceID, &e.CreatedAt)
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
```

- [ ] **Step 3: Update listExpanded to select and scan calendar_id, and support optional filtering**

Replace `HandleList` and `listExpanded`:

```go
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
	to = to.Add(24*time.Hour - time.Second)

	var calendarIDs []int64
	if raw := r.URL.Query().Get("calendar_ids"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
				calendarIDs = append(calendarIDs, id)
			}
		}
	}

	events, err := s.listExpanded(from, to, calendarIDs)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

func (s *Service) listExpanded(from, to time.Time, calendarIDs []int64) ([]map[string]interface{}, error) {
	query := `
		SELECT e.id, e.title, e.description, e.start_at, e.end_at, e.all_day,
		       e.creator_id, e.calendar_id, e.color_override, e.recurrence_id,
		       r.frequency, r.interval, r.weekdays, r.until
		FROM events e
		LEFT JOIN recurrence_rules r ON r.id = e.recurrence_id
		WHERE (e.recurrence_id IS NULL AND e.start_at BETWEEN ? AND ?)
		   OR  e.recurrence_id IS NOT NULL`

	args := []interface{}{from, to}

	if len(calendarIDs) > 0 {
		placeholders := strings.Repeat("?,", len(calendarIDs))
		placeholders = placeholders[:len(placeholders)-1]
		query += " AND e.calendar_id IN (" + placeholders + ")"
		for _, id := range calendarIDs {
			args = append(args, id)
		}
	}

	rows, err := s.db.Query(query, args...)
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
			&e.CreatorID, &e.CalendarID, &colorOverride, &recurrenceID,
			&rFreq, &rInterval, &rWeekdays, &rUntil,
		); err != nil {
			return nil, err
		}
		e.Description = description.String
		e.ColorOverride = colorOverride.String

		if !recurrenceID.Valid {
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
```

- [ ] **Step 4: Update HandleCreate to store calendar_id**

In `HandleCreate`, replace the INSERT statement:
```go
res, err := s.db.Exec(
    `INSERT INTO events (title, description, start_at, end_at, all_day, creator_id, calendar_id, color_override, recurrence_id)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    req.Title, req.Description, req.StartAt, req.EndAt, req.AllDay, u.ID, req.CalendarID, req.ColorOverride, recurrenceID,
)
```

- [ ] **Step 5: Update HandleUpdate to include calendar_id in all write paths**

In `HandleUpdate`, replace the INSERT inside `case "future":`:
```go
s.db.Exec(
    `INSERT INTO events (title, description, start_at, end_at, all_day, creator_id, calendar_id, color_override, recurrence_id)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    req.Title, req.Description, req.StartAt, req.EndAt, req.AllDay, e.CreatorID, e.CalendarID, req.ColorOverride, rid,
)
```

In `HandleUpdate`, replace the UPDATE inside `case "all", "":`:
```go
_, err = s.db.Exec(
    `UPDATE events SET title=?, description=?, start_at=?, end_at=?, all_day=?, calendar_id=?, color_override=? WHERE id=?`,
    req.Title, req.Description, req.StartAt, req.EndAt, req.AllDay, req.CalendarID, req.ColorOverride, baseID,
)
```

- [ ] **Step 6: Build**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add internal/calendar/calendar.go
git commit -m "feat: add calendar_id to event queries, create, update, and list"
```

---

### Task 5: Register calendar routes

**Files:**
- Modify: `internal/api/router.go`

- [ ] **Step 1: Add calendar routes**

In `internal/api/router.go`, update the router signature to accept `calSvc` (already present) and add inside the authenticated group, before the event routes:

```go
func NewRouter(authSvc *auth.Service, userSvc *user.Service, calSvc *calendar.Service) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/api/auth/login", authSvc.HandleLogin)
	r.Post("/api/auth/logout", authSvc.HandleLogout)
	r.Post("/api/auth/setup", authSvc.HandleSetup)

	r.Group(func(r chi.Router) {
		r.Use(authSvc.Middleware)

		r.Get("/api/auth/me", authSvc.HandleMe)

		r.Get("/api/calendars", calSvc.HandleListCalendars)
		r.Post("/api/calendars", auth.AdminOnly(http.HandlerFunc(calSvc.HandleCreateCalendar)).ServeHTTP)
		r.Put("/api/calendars/{id}", auth.AdminOnly(http.HandlerFunc(calSvc.HandleUpdateCalendar)).ServeHTTP)
		r.Delete("/api/calendars/{id}", auth.AdminOnly(http.HandlerFunc(calSvc.HandleDeleteCalendar)).ServeHTTP)

		r.Get("/api/events", calSvc.HandleList)
		r.Post("/api/events", calSvc.HandleCreate)
		r.Get("/api/events/{id}", calSvc.HandleGet)
		r.Put("/api/events/{id}", calSvc.HandleUpdate)
		r.Delete("/api/events/{id}", calSvc.HandleDelete)

		r.Get("/api/users", auth.AdminOnly(http.HandlerFunc(userSvc.HandleList)).ServeHTTP)
		r.Post("/api/users", auth.AdminOnly(http.HandlerFunc(userSvc.HandleCreate)).ServeHTTP)
		r.Put("/api/users/{id}", userSvc.HandleUpdate)
		r.Delete("/api/users/{id}", auth.AdminOnly(http.HandlerFunc(userSvc.HandleDelete)).ServeHTTP)
		r.Put("/api/users/me/preferences", userSvc.HandleUpdatePreferences)
		r.Put("/api/users/me/password", userSvc.HandleChangePassword)
	})

	return r
}
```

- [ ] **Step 2: Build and verify**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/api/router.go
git commit -m "feat: register /api/calendars routes"
```

---

### Task 6: Frontend — Alpine state, sidebar, and event form

**Files:**
- Modify: `cmd/server/web/index.html`

This task replaces `index.html` in full. The changes are:

**Alpine state additions** (in `app()` return object):
```js
calendars: [],
visibleCalendars: [],
sidebarOpen: true,
newCalendarForm: { show: false, name: '', color: '#6366f1' },
```

**New methods** (add after `toggleDark()`):
```js
async loadCalendars() {
    const r = await fetch('/api/calendars');
    if (r.ok) {
        this.calendars = await r.json();
        this.visibleCalendars = this.calendars.map(c => c.id);
    }
},

toggleCalendar(id) {
    if (this.visibleCalendars.includes(id)) {
        this.visibleCalendars = this.visibleCalendars.filter(c => c !== id);
    } else {
        this.visibleCalendars = [...this.visibleCalendars, id];
    }
},

toggleSidebar() {
    this.sidebarOpen = !this.sidebarOpen;
},

async createCalendar() {
    if (!this.newCalendarForm.name) return;
    const r = await fetch('/api/calendars', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: this.newCalendarForm.name, color: this.newCalendarForm.color }),
    });
    if (r.ok || r.status === 201) {
        this.newCalendarForm = { show: false, name: '', color: '#6366f1' };
        await this.loadCalendars();
    }
},

async deleteCalendar(id) {
    const r = await fetch(`/api/calendars/${id}`, { method: 'DELETE' });
    if (r.status === 409) {
        alert('Cannot delete a calendar that has events.');
        return;
    }
    if (r.ok || r.status === 204) {
        await this.loadCalendars();
    }
},
```

**Updated `init()` and `login()`** — call `await this.loadCalendars()` after setting `this.user`.

**Updated `logout()`** — reset: `this.calendars = []; this.visibleCalendars = [];`

**Updated `openNewEvent()`** — set `calendar_id` to first visible calendar:
```js
openNewEvent(date) {
    const calId = this.visibleCalendars[0] || (this.calendars[0] && this.calendars[0].id) || 0;
    this.eventForm = { id: null, occurrence_date: '', title: '', description: '',
        start_at: date + 'T09:00', end_at: date + 'T10:00', all_day: false,
        calendar_id: calId, color_override: '', is_recurring: false,
        scope: 'this', recurrence: { frequency: '', until: '' } };
    this.view = 'event';
},
```

**Updated `openEvent()`** — include `calendar_id: ev.calendar_id || 0`.

**Updated `saveEvent()`** — include `calendar_id: this.eventForm.calendar_id` in request body.

**Updated `eventsForDate()`**:
```js
eventsForDate(dateStr) {
    return this.events.filter(ev => {
        const d = (ev.occurrence_date || ev.start_at.slice(0, 10));
        return d === dateStr && this.visibleCalendars.includes(ev.calendar_id);
    });
},
```

**Layout change** — wrap calendar view content in a flex row with sidebar:

The main-app `<div>` structure becomes:
```html
<div x-show="user" class="max-w-5xl mx-auto px-4 py-6">
  <!-- Header (unchanged except toggle button) -->
  <header ...>
    ...
    <button @click="toggleSidebar()" ...>☰</button>
    ...
  </header>

  <!-- Content: sidebar + views -->
  <div class="flex gap-4">

    <!-- Sidebar (only shown on calendar view) -->
    <div x-show="sidebarOpen && view === 'calendar'"
         class="w-48 shrink-0">
      <div class="bg-amber-50 dark:bg-stone-800 border border-amber-200 dark:border-stone-700 rounded-xl p-3 shadow-sm">
        <p class="text-xs font-semibold text-stone-500 dark:text-stone-400 uppercase tracking-wide mb-2">Calendars</p>
        <template x-for="cal in calendars" :key="cal.id">
          <label class="flex items-center gap-2 py-1 cursor-pointer">
            <input type="checkbox"
                   :checked="visibleCalendars.includes(cal.id)"
                   @change="toggleCalendar(cal.id)"
                   class="rounded">
            <span class="w-3 h-3 rounded-full shrink-0" :style="'background:' + cal.color"></span>
            <span class="text-sm truncate" x-text="cal.name"></span>
            <button x-show="user && user.is_admin"
                    @click.prevent="deleteCalendar(cal.id)"
                    class="ml-auto text-stone-400 hover:text-red-500 text-xs leading-none">×</button>
          </label>
        </template>

        <!-- Admin: new calendar form -->
        <div x-show="user && user.is_admin" class="mt-3 border-t border-stone-200 dark:border-stone-700 pt-3">
          <div x-show="!newCalendarForm.show">
            <button @click="newCalendarForm.show = true"
                    class="text-xs text-violet-600 dark:text-violet-400 hover:underline">+ New Calendar</button>
          </div>
          <div x-show="newCalendarForm.show">
            <input type="text" x-model="newCalendarForm.name" placeholder="Name"
                   class="w-full bg-white dark:bg-stone-700 border dark:border-stone-600 rounded px-2 py-1 text-xs mb-1 focus:outline-none focus:ring-1 focus:ring-violet-500">
            <div class="flex gap-1 items-center">
              <input type="color" x-model="newCalendarForm.color" class="h-6 w-8 border rounded cursor-pointer">
              <button @click="createCalendar()"
                      class="flex-1 bg-violet-600 text-white rounded px-2 py-1 text-xs hover:bg-violet-700">Save</button>
              <button @click="newCalendarForm.show = false"
                      class="text-xs text-stone-400 hover:text-stone-600">✕</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Right column: calendar / event form / settings -->
    <div class="flex-1 min-w-0">
      <!-- (all three x-show view divs go here) -->
    </div>

  </div>
</div>
```

**Event form** — add a Calendar `<select>` above the Title field:
```html
<div class="mb-4">
  <label class="block text-sm font-medium mb-1">Calendar</label>
  <select x-model="eventForm.calendar_id"
          class="w-full bg-white dark:bg-stone-700 border dark:border-stone-600 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-violet-500">
    <template x-for="cal in calendars" :key="cal.id">
      <option :value="cal.id" x-text="cal.name"></option>
    </template>
  </select>
</div>
```

**Sidebar toggle button** in header — add between avatar dot and dark mode button:
```html
<button @click="toggleSidebar()" x-show="view === 'calendar'"
        class="text-sm text-stone-500 dark:text-stone-400 hover:text-stone-700 dark:hover:text-stone-200 transition"
        title="Toggle sidebar">☰</button>
```

- [ ] **Step 1: Apply all the above changes to `cmd/server/web/index.html`**

Make the changes described above. The complete restructured file must:
- Have the two-column flex layout wrapping the sidebar and right column
- Move all three view divs (calendar, event form, settings) into the right column `<div class="flex-1 min-w-0">`
- Include all new Alpine methods and updated existing methods

- [ ] **Step 2: Regenerate CSS and build**

```bash
make css && make build
```

Expected: both commands succeed with no errors.

- [ ] **Step 3: Manual smoke test**

```bash
./family-cal
```

Open browser at `http://localhost:8080`. Verify:
- Sidebar visible with "Activities" calendar and checkbox
- ☰ button in header toggles sidebar
- Creating an event shows Calendar dropdown
- Unchecking Activities hides its events immediately
- Admin sees `+` New Calendar button and `×` delete buttons

- [ ] **Step 4: Commit**

```bash
git add cmd/server/web/index.html
git commit -m "feat: multi-calendar sidebar, event form picker, and Alpine state"
```

---

### Task 7: Rebuild Docker image

**Files:** none (build only)

- [ ] **Step 1: Build updated image**

```bash
make d-build
```

Expected: `Successfully built ...` — image tagged `family-cal:latest`.

- [ ] **Step 2: Commit any generated changes**

```bash
git status
# commit if anything unexpected was modified
```
