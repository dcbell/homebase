# Homebase

Homebase is a household operations app scaffold with a Go API, separate server-rendered web frontend, PostgreSQL storage, Google login wiring, and a Docker deployment path.

## What Exists

- API service under `cmd/api`
- Web service under `cmd/web`
- PostgreSQL schema embedded in `internal/store/migrations`
- Household/user/session model with household data isolation
- Editable projects, project tasks, standalone tasks, assignees, appointments, day/week/month calendar, routines with automatic task generation, household lists, household contacts, uploaded documents with related-item links, movable dashboard tiles, and read-only supporting module endpoints
- In-app routine notices surfaced through the notification bell
- Google OAuth login wiring with calendar scope
- Dev-login fallback when Google credentials are not configured
- Pre-added user requirement for real Google login
- Budget app link via `BUDGET_APP_URL`

## Run Locally

```sh
cp .env.example .env
docker compose up --build
```

Then open:

- Web frontend: http://localhost:8080
- API health: http://localhost:8081/healthz

If Google OAuth is not configured, the login button redirects to a development login user. In production, set `APP_ENV=production`, configure Google OAuth credentials, and pre-add the first owner with `BOOTSTRAP_OWNER_EMAIL`.

## Google OAuth

Create a Google OAuth client and use this redirect URI for local development:

```text
http://localhost:8081/auth/google/callback
```

Set:

```sh
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GOOGLE_REDIRECT_URL=http://localhost:8081/auth/google/callback
```

The login flow requests profile, email, and calendar scopes. Calendar sync endpoints are reserved for the worker-backed two-way sync implementation.

Google login is open to start, but the callback only succeeds when the Google account email already belongs to a household. Use the Household Members tile to add more allowed users after the first owner logs in.

For a fresh production install, seed the first owner:

```sh
BOOTSTRAP_OWNER_EMAIL=you@example.com
BOOTSTRAP_OWNER_NAME="Your Name"
BOOTSTRAP_HOUSEHOLD_NAME="Home"
```

## Architecture Notes

- The frontend contains display and form forwarding only; business rules stay in the API.
- The app uses PostgreSQL for development and production to avoid SQLite/Postgres behavior drift.
- The backend is a modular monolith. Add new household modules behind the API before exposing them in the web frontend.
- Due routines are checked by the API process on startup and then every 15 minutes by default. Override with `ROUTINE_CHECK_INTERVAL_SECONDS`.
- Documents link to projects and tasks through a reusable related-item table so the same document can be reused from multiple places. Uploaded document files are stored by the API service under `DOCUMENT_UPLOAD_DIR`; Docker Compose persists them in the `homebase_uploads` volume. Override the upload limit with `DOCUMENT_MAX_UPLOAD_MB`.
