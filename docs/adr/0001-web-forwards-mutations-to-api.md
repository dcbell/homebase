# Web service forwards all mutations to the API

Homebase runs as two Go processes. `cmd/api` owns authentication, business rules, scheduling, and file storage. `cmd/web` only renders pages and forwards every mutation to the API. We chose this over a single process that renders and writes directly. It means browser sessions and bearer-token clients follow exactly the same rules, and the OpenAPI contract stays complete. Do not add business logic or direct database access to the web service, even when it would be quicker.
