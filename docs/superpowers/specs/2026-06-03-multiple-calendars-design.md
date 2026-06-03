# Multiple Calendars Design Spec

**Date:** 2026-06-03  
**Status:** Approved

## Summary

Administrators can create named calendars that are shared by all family members. Each calendar is an independent container of events with its own color. A collapsible sidebar lets users toggle individual calendars on/off to control what appears on the month grid. All users can create events on any calendar; only admins can create or delete calendars.

---

## Data Model

### New table: `calendars`

```sql
CREATE TABLE calendars (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL,
  color      TEXT NOT NULL DEFAULT '#6366f1',
  created_by INTEGER NOT NULL REFERENCES users(id),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Modified table: `events`

Add a non-nullable foreign key:

```sql
ALTER TABLE events ADD COLUMN calendar_id INTEGER NOT NULL REFERENCES calendars(id);
```

(Applied via migration after a default calendar is inserted — see below.)

### Default "Activities" calendar

Two code paths guarantee a default calendar always exists:

1. **First-run setup (`CreateAdmin`)** — immediately after inserting the admin user, insert the "Activities" calendar owned by that user.
2. **Migration `003_calendars.sql`** — for existing databases, insert "Activities" (owned by the earliest admin) if no calendars exist yet, then set `calendar_id` on all existing events to that calendar's ID.

A fresh install and an upgraded install both arrive at the same state without any user action.

---

## API

### New endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/calendars` | all users | List all calendars |
| `POST` | `/api/calendars` | admin only | Create a calendar |
| `PUT` | `/api/calendars/{id}` | admin only | Rename or recolor |
| `DELETE` | `/api/calendars/{id}` | admin only | Delete (blocked if calendar has events) |

**`GET /api/calendars` response:**
```json
[{"id":1,"name":"Activities","color":"#6366f1","created_by":1}]
```

**`POST /api/calendars` request body:**
```json
{"name":"Sports","color":"#22c55e"}
```

**`PUT /api/calendars/{id}` request body:** same shape, all fields optional.

**`DELETE /api/calendars/{id}`:** Returns 409 Conflict if the calendar contains any events.

### Modified endpoints

- `POST /api/events` — body gains required `calendar_id` field
- `GET /api/events` — accepts optional `?calendar_ids=1,2,3`; omitting returns all
- `PUT /api/events/{id}` — body may include `calendar_id` to move an event between calendars

### New file: `internal/calendar/calendar_crud.go`

Calendar CRUD handlers live here, separate from event logic in the existing `calendar.go`. Follows the same `Service`-based pattern.

---

## Frontend

### Layout

The main app area becomes a two-column flex layout:

```
[ sidebar ]  [ calendar grid ]
```

A chevron/hamburger button at the top-left of the sidebar toggles it. When collapsed, the calendar grid expands to full width.

### Sidebar

- "Calendars" heading + collapse button
- One row per calendar: colored dot · name · visibility checkbox
- Admin-only: `+` button → inline form (name input + color picker + Save)
- Admin-only: `×` delete button per row (shows alert if calendar has events)

### Alpine state additions

```js
calendars: [],          // [{id, name, color, created_by}]
visibleCalendars: [],   // array of calendar IDs currently checked
sidebarOpen: true,
```

`init()` and `login()` load calendars from `GET /api/calendars` and initialize `visibleCalendars` to all calendar IDs (all shown by default).

### Event filtering

`eventsForDate(dateStr)` filters to events whose `calendar_id` is in `visibleCalendars`. Toggling a calendar checkbox adds/removes its ID from `visibleCalendars` — no API refetch needed.

### Event form

A "Calendar" `<select>` is added above the Title field. Required. Pre-selects the first calendar in `calendars[]`. `eventForm` gains a `calendar_id` property.

### Event coloring

Unchanged — events remain colored by the creator's user color. The calendar name is not shown on the chip (title already truncates on small cells).

### Settings page (admin)

The Family Members card keeps its existing form. Calendar management is handled entirely in the sidebar, not in Settings.

---

## Verification

```bash
make css && make build && ./family-cal
```

Check:
- [ ] Fresh install: "Activities" calendar exists after setup, no extra steps required
- [ ] Existing DB: migration creates "Activities" and assigns all events to it
- [ ] Non-admin: can view all calendars, cannot see `+` or `×` controls in sidebar
- [ ] Admin: can create a calendar with a name and color
- [ ] Admin: cannot delete a calendar that has events (409 response, alert shown)
- [ ] Admin: can delete an empty calendar
- [ ] Event form: Calendar dropdown present, required, pre-selected
- [ ] Creating an event assigns it to the selected calendar
- [ ] Unchecking a calendar in sidebar hides its events immediately (no reload)
- [ ] Sidebar collapse button hides sidebar, calendar expands to full width
- [ ] Sidebar re-open restores calendar list and checked state
- [ ] Dark mode: sidebar and calendar rows render correctly in both modes
