# Dark Mode Design Spec

**Date:** 2026-06-03  
**Status:** Approved

## Summary

Add a toggleable dark mode to the family calendar app. The preference is stored server-side per user, survives page reloads, and syncs across devices. A sun/moon icon button in the header controls the toggle.

---

## Color Palette

| Role | Light | Dark |
|------|-------|------|
| Page background | `bg-stone-100` | `dark:bg-stone-900` |
| Card background | `bg-amber-50` | `dark:bg-stone-800` |
| Card border | `border-amber-200` | `dark:border-stone-700` |
| Header background | `bg-stone-50` | `dark:bg-stone-900` |
| Calendar cells | `bg-stone-50` | `dark:bg-stone-800` |
| Calendar gap | `bg-stone-300` | `dark:bg-stone-700` |
| Inputs | `bg-white` | `dark:bg-stone-700` |
| Text primary | `text-stone-900` | `dark:text-stone-100` |
| Text secondary | `text-stone-600/700` | `dark:text-stone-300` |
| Text muted | `text-stone-500` | `dark:text-stone-400` |
| Borders / dividers | `border-stone-200` | `dark:border-stone-700` |
| Accent links/text | `text-violet-600` | `dark:text-violet-400` |
| Accent buttons | `bg-violet-600 hover:bg-violet-700` | unchanged |
| Hover (calendar cells) | `hover:bg-violet-50` | `dark:hover:bg-stone-700` |
| Hover (nav arrows) | `hover:bg-stone-200` | `dark:hover:bg-stone-700` |
| Delete button | `bg-red-50 text-red-600 border-red-200` | `dark:bg-red-900/30 dark:text-red-400 dark:border-red-800` |

---

## Backend

### Migration: `internal/db/migrations/002_dark_mode.sql`

```sql
ALTER TABLE users ADD COLUMN dark_mode INTEGER NOT NULL DEFAULT 0;
```

### User struct

Add `DarkMode bool \`json:"dark_mode"\`` to the `User` struct (in `internal/user/` or `internal/auth/`).

Include `dark_mode` in all user JSON responses: `GET /api/auth/me`, `POST /api/auth/login`, `POST /api/auth/setup`.

### New endpoint: `PUT /api/users/me/preferences`

**Request body:**
```json
{"dark_mode": true}
```

**Behaviour:** Updates `dark_mode` for the authenticated user. Returns 204 No Content.  
**Auth:** Requires valid JWT cookie (existing auth middleware).  
**Route registration:** Add to the existing chi router in `internal/api/`.

---

## Frontend

### Tailwind config (`cmd/server/web/input.css`)

Add one line to enable class-based dark mode:
```css
@custom-variant dark (&:where(.dark, .dark *));
```

### Alpine.js (`app()` in `index.html`)

**New property:**
```js
darkMode: false,
```

**Initialization** — after user loads (in `init()` and `login()`):
```js
this.darkMode = this.user.dark_mode || false;
document.documentElement.classList.toggle('dark', this.darkMode);
```

**New method:**
```js
async toggleDark() {
  this.darkMode = !this.darkMode;
  document.documentElement.classList.toggle('dark', this.darkMode);
  await fetch('/api/users/me/preferences', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ dark_mode: this.darkMode }),
  });
},
```

Also clear the class on logout:
```js
document.documentElement.classList.remove('dark');
```

### Toggle button (in header)

Between the avatar dot and the Settings link:
```html
<button @click="toggleDark()" class="text-sm text-stone-500 hover:text-stone-700 dark:text-stone-400 dark:hover:text-stone-200 transition" x-text="darkMode ? '☀' : '🌙'"></button>
```

### HTML class pairs

Every element with a color class in `index.html` gets a `dark:` partner per the palette table above.

---

## Verification

```bash
make css    # picks up @custom-variant dark
make build  # embeds updated HTML
./family-cal
```

Check:
- [ ] Toggle button visible in header (moon icon in light, sun in dark)
- [ ] Clicking toggle switches theme immediately
- [ ] Page background, cards, header all shift to dark palette
- [ ] Calendar cells, event chips, today ring all correct in dark mode
- [ ] Preference saved: refresh page → correct mode restored
- [ ] Login as different user with opposite preference → their mode loads
- [ ] Delete button readable in dark mode
- [ ] Inputs readable (stone-700 bg) with correct border color
