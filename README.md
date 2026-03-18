# PharmaLink Backend (Go Fiber)

Backend service for PharmaLink B2B pharma marketplace.

## Stack
- Go + Fiber
- PostgreSQL + GORM
- Redis + Asynq
- `golang-migrate` for migrations
- Outbox pattern (`outbox` table + Postgres `LISTEN/NOTIFY`)
- SSE (`/api/v1/stream/offers`)
- JWT auth (access + refresh)
- Zerolog structured logs
- Docker (`api`, `worker`, `postgres`, `redis`, `nginx`)

## Run
See full run instructions in [docs/RUN.md](docs/RUN.md).

## Swagger
- UI: `GET /api/v1/docs/index.html`
- JSON: `GET /api/v1/docs/swagger.json`

Generate docs:
```bash
go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go -o internal/docs
```

## Migration Commands
```bash
# Up
migrate -path migrations -database "postgres://pharmalink:pharmalink@localhost:5432/pharmalink?sslmode=disable" up

# Down 1 step
migrate -path migrations -database "postgres://pharmalink:pharmalink@localhost:5432/pharmalink?sslmode=disable" down 1
```

## Environment Variables
Copy from `.env.example`. Important variables:
- `HTTP_ADDR`
- `CORS_ALLOWED_ORIGINS`
- `JWT_ACCESS_SECRET`
- `JWT_REFRESH_SECRET`
- `PAYMENT_WEBHOOK_SECRET`
- `POSTGRES_*`
- `REDIS_*`
- `OUTBOX_CHANNEL`

## API Base Path
- `/api/v1`

## Related Docs
- [Run Guide](docs/RUN.md)
- [Developer Guide](docs/DEVELOPER.md)
- [API Testing](docs/API_TESTING.md)
- [Frontend Catalog Import](docs/FRONTEND_CATALOG_IMPORT.md)
