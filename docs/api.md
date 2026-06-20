# API Documentation

The API contract is documented as OpenAPI 3.0 in `internal/api/openapi.yaml`.

When the web service is running, Swagger UI is available at:

- `GET /docs`

The raw OpenAPI document is also available through the web service at:

- `GET /docs/openapi.yaml`
- `GET /openapi.yaml`

The API service also serves the same raw document internally at:

- `GET /openapi.yaml`
- `GET /docs/openapi.yaml`

All `/api/v1` routes require either a valid `homebase_session` cookie or an
`Authorization: Bearer hb_...` API token unless otherwise noted in the OpenAPI
document. Read-only tokens cannot call write operations.
