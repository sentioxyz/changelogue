.PHONY: up down db-reset build cli run run-auth dev test vet lint coverage \
        frontend-install frontend-dev frontend-build \
        integration-test agent-dev clean

# --- Configuration ---
DATABASE_URL  ?= postgres://postgres:postgres@localhost:5432/releaseguard?sslmode=disable
LISTEN_ADDR   ?= :8080
FRONTEND_URL  ?= http://localhost:3001
BINARY        := changelogue
VERSION       ?= dev

# --- Infrastructure ---
up:
	docker compose up -d
	@echo "Waiting for Postgres to be healthy..."
	@until docker compose exec -T postgres pg_isready -U postgres >/dev/null 2>&1; do sleep 1; done
	@echo "Postgres is ready."

down:
	docker compose down

db-reset:
	docker compose exec -T postgres psql -U postgres -c "DROP DATABASE IF EXISTS changelogue;"
	docker compose exec -T postgres psql -U postgres -c "CREATE DATABASE changelogue;"
	@echo "Database reset."

# --- Backend ---
build:
	go build -o $(BINARY) ./cmd/server

cli:
	go build -ldflags "-X main.version=$(VERSION)" -o clog ./cmd/cli

run: build
	DATABASE_URL="$(DATABASE_URL)" LISTEN_ADDR="$(LISTEN_ADDR)" FRONTEND_URL="$(FRONTEND_URL)" NO_AUTH=true ./$(BINARY)

run-auth: build
	DATABASE_URL="$(DATABASE_URL)" \
	LISTEN_ADDR="$(LISTEN_ADDR)" \
	SECURE_COOKIES=false \
	GITHUB_CLIENT_ID="$(GITHUB_CLIENT_ID)" \
	GITHUB_CLIENT_SECRET="$(GITHUB_CLIENT_SECRET)" \
	ALLOWED_GITHUB_USERS="$(ALLOWED_GITHUB_USERS)" \
	ALLOWED_GITHUB_ORGS="$(ALLOWED_GITHUB_ORGS)" \
	SESSION_SECRET="$(SESSION_SECRET)" \
	FRONTEND_URL="$(FRONTEND_URL)" \
	./$(BINARY)

test:
	go test ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | grep total

vet:
	go vet ./...

lint: vet

# --- Agent Dev ---
agent-dev:
	@if [ -z "$(PROJECT_ID)" ]; then echo "error: PROJECT_ID is required. Usage: make agent-dev PROJECT_ID=<uuid>"; exit 1; fi
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/agent --project-id=$(PROJECT_ID) web api webui

# --- Frontend ---
frontend-install:
	cd web && npm install

frontend-dev:
	cd web && npm run dev

frontend-build:
	cd web && npm run build

# --- Integration ---
integration-test:
	bash scripts/integration-test.sh

# --- Convenience ---
dev: up run

clean:
	rm -f $(BINARY)
	docker compose down -v
