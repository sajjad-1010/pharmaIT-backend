APP_NAME=pharmalink

.PHONY: help
help:
	@echo "Targets:"
	@echo "  up              Start docker compose stack"
	@echo "  down            Stop docker compose stack"
	@echo "  logs            Tail compose logs"
	@echo "  migrate-up      Run migrations up (local host db)"
	@echo "  migrate-down    Rollback one migration"
	@echo "  swagger         Generate swagger docs"
	@echo "  run-api         Run API service locally"
	@echo "  run-worker      Run worker service locally"

.PHONY: up
up:
	docker compose up -d --build

.PHONY: down
down:
	docker compose down

.PHONY: logs
logs:
	docker compose logs -f --tail=150

.PHONY: migrate-up
migrate-up:
	migrate -path migrations -database "postgres://$${POSTGRES_USER:-pharmalink}:$${POSTGRES_PASSWORD:-pharmalink}@$${POSTGRES_HOST:-localhost}:$${POSTGRES_PORT:-5432}/$${POSTGRES_DB:-pharmalink}?sslmode=$${POSTGRES_SSLMODE:-disable}" up

.PHONY: migrate-down
migrate-down:
	migrate -path migrations -database "postgres://$${POSTGRES_USER:-pharmalink}:$${POSTGRES_PASSWORD:-pharmalink}@$${POSTGRES_HOST:-localhost}:$${POSTGRES_PORT:-5432}/$${POSTGRES_DB:-pharmalink}?sslmode=$${POSTGRES_SSLMODE:-disable}" down 1

.PHONY: swagger
swagger:
	swag init -g cmd/api/main.go -o internal/docs

.PHONY: run-api
run-api:
	go run ./cmd/api

.PHONY: run-worker
run-worker:
	go run ./cmd/worker

