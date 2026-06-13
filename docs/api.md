# API Sketch

All `/api/v1` routes require a valid `homebase_session` cookie.

## Auth

- `GET /auth/google/start`
- `GET /auth/google/callback`
- `GET /auth/dev-login` development only
- `POST /auth/logout`

## Household

- `GET /api/v1/me`
- `GET /api/v1/households/current`
- `GET /api/v1/dashboard`
  - Returns household dashboard data, active `tile_order`, and `available_tiles`.
- `GET /api/v1/calendar?month=YYYY-MM`
- `POST /api/v1/dashboard/tiles/move`
- `POST /api/v1/dashboard/tiles/order`
  - Accepts `{ "tiles": ["calendar", "tasks"] }`; omitted available tiles are hidden until re-added.
- `GET /api/v1/members`
- `POST /api/v1/members`
- `PATCH /api/v1/members/{id}`
- `DELETE /api/v1/members/{id}`

## Operations

- `GET /api/v1/projects`
- `POST /api/v1/projects`
- `GET /api/v1/projects/{id}`
- `PATCH /api/v1/projects/{id}`
- `POST /api/v1/projects/{id}/archive`
- `GET /api/v1/projects/{id}/folders`
- `POST /api/v1/projects/{id}/folders`
- `PATCH /api/v1/project-folders/{id}`
- `POST /api/v1/project-folders/{id}/archive`
- `GET /api/v1/tasks`
- `POST /api/v1/tasks`
- `GET /api/v1/tasks/{id}`
- `PATCH /api/v1/tasks/{id}`
- `PATCH /api/v1/tasks/{id}/complete`
- `PATCH /api/v1/tasks/{id}/reopen`
- `POST /api/v1/tasks/{id}/archive`
- `GET /api/v1/events`
- `POST /api/v1/events`
- `GET /api/v1/events/{id}`
- `PATCH /api/v1/events/{id}`
- `DELETE /api/v1/events/{id}`
- `GET /api/v1/routines`
- `POST /api/v1/routines`
- `GET /api/v1/routines/{id}`
- `PATCH /api/v1/routines/{id}`
- `POST /api/v1/routines/{id}/archive`
- `POST /api/v1/routines/{id}/generate-task` internal/manual fallback; normal task generation is automatic.
- `GET /api/v1/lists`
- `POST /api/v1/lists`
- `GET /api/v1/lists/{id}`
- `PATCH /api/v1/lists/{id}`
- `POST /api/v1/lists/{id}/archive`
- `GET /api/v1/lists/{id}/items`
- `POST /api/v1/lists/{id}/items`
- `PATCH /api/v1/list-items/{id}`
- `PATCH /api/v1/list-items/{id}/complete`
- `PATCH /api/v1/list-items/{id}/reopen`
- `POST /api/v1/list-items/{id}/archive`
- `GET /api/v1/contacts`
- `POST /api/v1/contacts`
- `GET /api/v1/contacts/{id}`
- `PATCH /api/v1/contacts/{id}`
- `POST /api/v1/contacts/{id}/archive`
- `GET /api/v1/related-contacts?entity_type=project|task&entity_id={id}`
- `POST /api/v1/related-contacts`
- `DELETE /api/v1/contact-links/{id}`
- `GET /api/v1/documents`
- `POST /api/v1/documents`
  - Accepts JSON metadata for API clients or multipart form data with `file` for uploads.
- `GET /api/v1/documents/{id}`
- `PATCH /api/v1/documents/{id}`
- `GET /api/v1/documents/{id}/download`
- `POST /api/v1/documents/{id}/archive`
- `GET /api/v1/documents/{id}/related-items`
- `POST /api/v1/documents/{id}/related-items`
- `GET /api/v1/related-documents?entity_type=project|task&entity_id={id}`
- `POST /api/v1/related-documents`
- `DELETE /api/v1/document-links/{id}`

## Supporting Modules

- `GET /api/v1/modules/routines`
- `GET /api/v1/modules/lists`
- `GET /api/v1/modules/contacts`
- `GET /api/v1/modules/assets`
- `GET /api/v1/modules/documents`

## Google Calendar

- `POST /api/v1/integrations/google/calendar/connect`
- `POST /api/v1/integrations/google/calendar/sync`
