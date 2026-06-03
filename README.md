# Family Calendar

A self-hosted family calendar. Single Go binary with an embedded SQLite database and an Alpine.js frontend — no external services required except an optional SMTP relay for event reminders.

## Features

- Shared calendar where each family member's events are color-coded by person
- Recurring events (daily / weekly / monthly / yearly) with per-occurrence editing
- Email reminders sent before upcoming events
- Dark mode with per-user preference
- Admin controls to add and remove family members

## Running with Docker Compose

The recommended way to run the app:

```bash
cp .env.example .env   # then edit .env with your real values
docker compose up -d
```

This builds the image, starts the container, mounts `./data` for the database, and restarts automatically on reboot.

To view logs or stop:

```bash
docker compose logs -f
docker compose down
```

## Running with Docker directly

```bash
docker run -p 8080:8080 \
  -v ./data:/data \
  -e JWT_SECRET=${JWT_SECRET:-change-me-to-a-random-secret} \
  -e SMTP_HOST=${SMTP_HOST:-smtp.example.com} \
  -e SMTP_PORT=${SMTP_PORT:-587} \
  -e SMTP_USER=${SMTP_USER:-user@example.com} \
  -e SMTP_PASS=${SMTP_PASS:-secret} \
  -e SMTP_FROM=${SMTP_FROM:-family-cal@example.com} \
  family-cal:latest
```

Or via make (uses the same defaults):

```bash
make d-run
```

The database is stored at `/data/family-cal.db` inside the container. Mount a volume there to persist it across restarts.

## Building from Source

**Prerequisites:** Go 1.26+, [Tailwind CSS standalone CLI](https://github.com/tailwindlabs/tailwindcss/releases) in `PATH`

```bash
make build       # generate CSS then compile → ./family-cal
make dev         # watch CSS + run server
make d-build     # build docker image
make css         # regenerate CSS only
```

## Configuration

Copy `config.yaml.example` to `config.yaml` and fill in your values, or set the equivalent environment variables:

| YAML key | Env var | Default | Description |
|----------|---------|---------|-------------|
| `server_port` | `SERVER_PORT` | `8080` | HTTP listen port |
| `db_path` | `DB_PATH` | `./family-cal.db` | SQLite database path |
| `smtp_host` | `SMTP_HOST` | — | SMTP server hostname |
| `smtp_port` | `SMTP_PORT` | — | SMTP port (typically 587) |
| `smtp_user` | `SMTP_USER` | — | SMTP username |
| `smtp_pass` | `SMTP_PASS` | — | SMTP password |
| `smtp_from` | `SMTP_FROM` | — | From address for reminder emails |
| `jwt_secret` | `JWT_SECRET` | — | Secret for signing session tokens — set this to a long random string |

SMTP config is optional. If omitted, the notification scheduler starts but silently skips sending.

## First-Run Setup

On first start with an empty database, the server prints a one-time setup token to stdout:

```
=== FIRST RUN SETUP ===
Setup token: abc123...
POST /api/auth/setup with {"token":"abc123...","email":"...","name":"...","password":"..."}
======================
```

Use that token to create the initial admin account:

```bash
curl -X POST http://localhost:8080/api/auth/setup \
  -H "Content-Type: application/json" \
  -d '{"token":"<token>","email":"you@example.com","name":"Your Name","password":"yourpassword"}'
```

The admin can then add other family members from the Settings page.

## Architecture

```
cmd/server/          Go entry point + embedded frontend (HTML, CSS, JS)
internal/
  auth/              JWT auth, bcrypt, login/logout/setup handlers
  calendar/          Event CRUD, recurrence expansion, occurrence exceptions
  user/              Family member management, color, notification prefs
  notification/      Hourly scheduler, SMTP sender, deduplication log
  db/                SQLite connection (WAL mode), embedded migration runner
  api/               chi router wiring all handlers + config loader
```

DB migrations in `internal/db/migrations/` are applied automatically on startup.
