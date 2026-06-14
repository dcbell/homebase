# Homebase

Homebase is a household operations app scaffold with a Go API, separate server-rendered web frontend, PostgreSQL storage, configurable OAuth/OIDC login wiring, and a Docker deployment path.

## What Exists

- API service under `cmd/api`
- Web service under `cmd/web`
- PostgreSQL schema embedded in `internal/store/migrations`
- Household/user/session model with household data isolation
- Editable projects, project tasks, standalone tasks, assignees, appointments, day/week/month calendar, routines with automatic task generation, household lists, household contacts, uploaded documents with related-item links, movable dashboard tiles, and read-only supporting module endpoints
- In-app routine notices surfaced through the notification bell
- Configurable OAuth/OIDC login wiring
- Dev-login fallback when OAuth credentials are not configured
- Pre-added user requirement for real OAuth login
- Budget app link via `BUDGET_APP_URL`

## Run Locally

```sh
cp .env.example .env
docker compose up --build
```

Then open:

- Web frontend: http://localhost:8080
- API health: http://localhost:8081/healthz
- Swagger UI: http://localhost:8080/docs

If OAuth is not configured, the login button redirects to a development login user. In production, set `APP_ENV=production`, configure OAuth/OIDC credentials, and pre-add the first owner with `BOOTSTRAP_OWNER_EMAIL`.

## Docker Hub Deploy Example

An example compose file for the published Docker Hub images is available at `docker-compose.dockerhub.example.yml`.

Set at least these values on the test server:

```sh
POSTGRES_PASSWORD=...
SESSION_SECRET=...
WEB_BASE_URL=https://homebase.example.com
API_BASE_URL=https://homebase.example.com
OAUTH_PROVIDER_NAME=authentik
OAUTH_ISSUER_URL=https://auth.example.com/application/o/homebase/
OAUTH_CLIENT_ID=...
OAUTH_CLIENT_SECRET=...
BOOTSTRAP_OWNER_EMAIL=you@example.com
```

Then run:

```sh
docker compose -f docker-compose.dockerhub.example.yml up -d
```

The compose file includes nginx as the public entrypoint. It routes app pages to the web container and `/auth/`, `/api/v1/`, `/healthz`, and OpenAPI paths to the API container. Publish that nginx service behind your public hostname so the web app and OAuth callback use the same origin.

The OAuth callback URL should be `${API_BASE_URL}/auth/oauth/callback` unless `OAUTH_REDIRECT_URL` is explicitly set.

## OAuth/OIDC

For an OIDC provider, configure the issuer URL and client credentials:

```text
http://localhost:8081/auth/oauth/callback
```

Set:

```sh
OAUTH_PROVIDER_NAME=internal
OAUTH_ISSUER_URL=https://auth.example.com
OAUTH_CLIENT_ID=...
OAUTH_CLIENT_SECRET=...
OAUTH_REDIRECT_URL=http://localhost:8081/auth/oauth/callback
OAUTH_SCOPES="openid profile email"
```

If your provider does not expose an OIDC discovery document, set the endpoints directly:

```sh
OAUTH_AUTH_URL=https://auth.example.com/oauth/authorize
OAUTH_TOKEN_URL=https://auth.example.com/oauth/token
OAUTH_USERINFO_URL=https://auth.example.com/oauth/userinfo
```

Login is open to start, but the callback only succeeds when the OAuth account email already belongs to a household. Use the Household Members screen to add more allowed users after the first owner logs in.

For a fresh production install, seed the first owner:

```sh
BOOTSTRAP_OWNER_EMAIL=you@example.com
BOOTSTRAP_OWNER_NAME="Your Name"
BOOTSTRAP_HOUSEHOLD_NAME="Home"
```

## API Tokens

Signed-in users can create API tokens from the profile menu under Settings. Tokens inherit the user's current household access and can be created as read-only or full-access tokens. The plaintext token is shown only once.

Use tokens with API requests:

```sh
curl -H "Authorization: Bearer hb_..." https://homebase.example.com/api/v1/me
```

Read-only tokens can call read endpoints only. Full-access tokens can call write endpoints, but token management itself still requires a browser session.

## Architecture Notes

- The frontend contains display and form forwarding only; business rules stay in the API.
- The app uses PostgreSQL for development and production to avoid SQLite/Postgres behavior drift.
- The backend is a modular monolith. Add new household modules behind the API before exposing them in the web frontend.
- Due routines are checked by the API process on startup and then every 15 minutes by default. Override with `ROUTINE_CHECK_INTERVAL_SECONDS`.
- Documents link to projects and tasks through a reusable related-item table so the same document can be reused from multiple places. Uploaded document files are stored by the API service under `DOCUMENT_UPLOAD_DIR`; Docker Compose persists them in the `homebase_uploads` volume. Override the upload limit with `DOCUMENT_MAX_UPLOAD_MB`.
