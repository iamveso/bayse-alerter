ENV_FILE ?= .env

ifneq (,$(wildcard $(ENV_FILE)))
include $(ENV_FILE)
export
endif

APP_NAME ?= bayse-alerter
MAIN_PACKAGE ?= ./cmd
BIN_DIR ?= bin
BIN_PATH ?= $(BIN_DIR)/$(APP_NAME)
TEST_PACKAGES ?= ./...
TEST_FLAGS ?= -v
COMPOSE ?= podman compose

DB_CONTAINER ?= bayse-alerter-postgres
CONTAINER_RUNTIME ?= podman
DB_IMAGE ?= docker.io/library/postgres:16-alpine
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_NAME ?= bayse_alerter
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DATABASE_URL ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

MIGRATIONS_DIR ?= internal/repository/migrations
MIGRATION_NAME ?= change
MIGRATE ?= go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

.PHONY: help run build test clean compose-up compose-down compose-logs postgres-up postgres-down postgres-logs postgres-shell migrate-up migrate-down migrate-force migrate-version migrate-create

help:
	@printf "Available targets:\n"
	@printf "  make run              Run the Go app\n"
	@printf "  make build            Build the Go app into $(BIN_PATH)\n"
	@printf "  make test             Run Go tests\n"
	@printf "  make clean            Remove build output\n"
	@printf "  make compose-up       Start app, Postgres, and migrations with Podman Compose\n"
	@printf "  make compose-down     Stop the Podman Compose stack\n"
	@printf "  make compose-logs     Tail Podman Compose logs\n"
	@printf "  make postgres-up      Start a local Postgres container\n"
	@printf "  make postgres-down    Stop and remove the Postgres container\n"
	@printf "  make postgres-logs    Tail Postgres logs\n"
	@printf "  make postgres-shell   Open psql in the Postgres container\n"
	@printf "  make migrate-up       Run all pending migrations\n"
	@printf "  make migrate-down     Roll back one migration\n"
	@printf "  make migrate-version  Show current migration version\n"
	@printf "  make migrate-create MIGRATION_NAME=create_users\n"

run:
	go run $(MAIN_PACKAGE)

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_PATH) $(MAIN_PACKAGE)

test:
	go test $(TEST_FLAGS) $(TEST_PACKAGES)

clean:
	rm -rf $(BIN_DIR)

compose-up:
	$(COMPOSE) up --build

compose-down:
	$(COMPOSE) down

compose-logs:
	$(COMPOSE) logs -f

postgres-up:
	$(CONTAINER_RUNTIME) run --name $(DB_CONTAINER) \
		-e POSTGRES_USER=$(DB_USER) \
		-e POSTGRES_PASSWORD=$(DB_PASSWORD) \
		-e POSTGRES_DB=$(DB_NAME) \
		-p $(DB_PORT):5432 \
		-d $(DB_IMAGE)

postgres-down:
	$(CONTAINER_RUNTIME) rm -f $(DB_CONTAINER)

postgres-logs:
	$(CONTAINER_RUNTIME) logs -f $(DB_CONTAINER)

postgres-shell:
	$(CONTAINER_RUNTIME) exec -it $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_NAME)

migrate-up:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" up

migrate-down:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" down 1

migrate-force:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" force $(VERSION)

migrate-version:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DATABASE_URL)" version

migrate-create:
	mkdir -p $(MIGRATIONS_DIR)
	$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -seq $(MIGRATION_NAME)

sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate
