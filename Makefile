.PHONY: up down db-reset build run dev test vet lint \
        frontend-install frontend-dev frontend-build \
        integration-test clean

# --- Configuration ---
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/changelogue?sslmode=disable
LISTEN_ADDR  ?= :8080
BINARY       := changelogue

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

run: build
	DATABASE_URL="$(DATABASE_URL)" LISTEN_ADDR="$(LISTEN_ADDR)" NO_AUTH=true ./$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

lint: vet

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
