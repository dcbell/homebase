FROM golang:1.23-bookworm AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /out/homebase-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /out/homebase-web ./cmd/web

FROM debian:bookworm-slim AS api

RUN useradd --system --uid 10001 --home /nonexistent homebase
RUN mkdir -p /var/lib/homebase/uploads && chown -R homebase:homebase /var/lib/homebase
COPY --from=build /out/homebase-api /usr/local/bin/homebase-api
USER homebase
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/homebase-api"]

FROM debian:bookworm-slim AS web

RUN useradd --system --uid 10001 --home /nonexistent homebase
COPY --from=build /out/homebase-web /usr/local/bin/homebase-web
USER homebase
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/homebase-web"]
