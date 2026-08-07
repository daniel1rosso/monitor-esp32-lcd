.DEFAULT_GOAL := help

.PHONY: help contracts backend frontend firmware-host compose-config docker-build up prod-config prod-up down logs dev check

help:
	@echo "Desk Monitor Platform"
	@echo "  make contracts     Validate OpenAPI, AsyncAPI, schemas and links"
	@echo "  make backend       Test and vet the Go scaffold"
	@echo "  make frontend      Generate API client, lint, test and build"
	@echo "  make firmware-host Build and test portable firmware contracts"
	@echo "  make compose-config Validate the resolved Compose model"
	@echo "  make docker-build   Build backend and frontend images"
	@echo "  make up             Build and start the local stack"
	@echo "  make prod-config    Validate production Compose and environment"
	@echo "  make prod-up        Build and start the production stack"
	@echo "  make down           Stop the stack without deleting data"
	@echo "  make logs           Follow stack logs"
	@echo "  make dev            Start with Air and Vite development overrides"
	@echo "  make check         Run the complete local stage gate"

contracts:
	npm ci --prefix tools/contracts
	npm run validate --prefix tools/contracts

backend:
	mkdir -p .cache/go .cache/tmp
	cd backend && GOCACHE="$(CURDIR)/.cache/go" GOTMPDIR="$(CURDIR)/.cache/tmp" go test -race ./... && GOCACHE="$(CURDIR)/.cache/go" GOTMPDIR="$(CURDIR)/.cache/tmp" go vet ./...

frontend:
	cd frontend && npm ci && npm run generate:api && npm run lint && npm test && npm run build

firmware-host:
	$(MAKE) -C firmware/host BUILD_DIR=/tmp/desk-firmware-host test

compose-config:
	docker compose config --quiet
	docker compose -f docker-compose.yml -f docker-compose.dev.yml config --quiet
	docker compose -f docker-compose.yml -f docker-compose.prod.yml config --quiet

docker-build:
	docker compose build backend frontend

up:
	docker compose up -d --build

prod-config:
	docker compose --env-file .env.production -f docker-compose.yml -f docker-compose.prod.yml config --quiet

prod-up:
	docker compose --env-file .env.production -f docker-compose.yml -f docker-compose.prod.yml up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

dev:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build

check: contracts backend frontend firmware-host compose-config
