# Run Guide

## Quick Start (Recommended)
1. Copy env:
```bash
cp .env.example .env
```
2. Start dependencies:
```bash
docker compose up -d postgres redis migrate
```
3. Run API:
```bash
go run ./cmd/api
```
4. Run Worker (new terminal):
```bash
go run ./cmd/worker
```

## Full Docker Stack
```bash
docker compose up -d --build
```

## Public Access (HTTP)
Run full stack (includes Nginx on port 80):
```bash
docker compose up -d --build
```

Public API base URL:
- `http://<YOUR_PUBLIC_IP>/api/v1`

## Public Access (HTTPS-ready)
1. Put your TLS certs in:
- `deploy/nginx/certs/fullchain.pem`
- `deploy/nginx/certs/privkey.pem`

2. Start with public override:
```bash
docker compose -f docker-compose.yml -f docker-compose.public.yml up -d --build
```

Public API base URL:
- `https://<YOUR_PUBLIC_IP>/api/v1`

Get your current public IP:
```bash
curl https://api.ipify.org
```

## Health Check
```bash
curl http://localhost:8080/health
```

## Swagger
- UI: `http://localhost:8080/api/v1/docs`
- JSON: `http://localhost:8080/api/v1/docs/swagger.json`

When using Nginx:
- UI: `http://localhost/api/v1/docs`
- JSON: `http://localhost/api/v1/docs/swagger.json`

If Swagger is empty:
```bash
swag init -g cmd/api/main.go -o internal/docs
```

## Stop
```bash
docker compose down
```
