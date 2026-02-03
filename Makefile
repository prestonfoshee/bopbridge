.PHONY: help build up down restart rebuild logs logs-api logs-db logs-redis shell psql redis-cli clean migrate-up migrate-down migrate-create migrate-force test

# Default target
help:
	@echo "BopBridge Development Commands"
	@echo "==============================="
	@echo ""
	@echo "Docker Commands:"
	@echo "  make build          - Build Docker images"
	@echo "  make up             - Start all services"
	@echo "  make down           - Stop all services"
	@echo "  make restart        - Restart all services (no rebuild)"
	@echo "  make rebuild        - Rebuild images and restart services"
	@echo "  make clean          - Stop services and remove volumes"
	@echo ""
	@echo "Logs:"
	@echo "  make logs           - View logs from all services"
	@echo "  make logs-api       - View API logs"
	@echo "  make logs-db        - View PostgreSQL logs"
	@echo "  make logs-redis     - View Redis logs"
	@echo ""
	@echo "Shell Access:"
	@echo "  make shell          - Access API container shell"
	@echo "  make psql           - Access PostgreSQL shell"
	@echo "  make redis-cli      - Access Redis CLI"
	@echo ""
	@echo "Database Migrations:"
	@echo "  make migrate-up     - Run all pending migrations"
	@echo "  make migrate-down   - Rollback last migration"
	@echo "  make migrate-create - Create new migration (NAME=migration_name)"
	@echo "  make migrate-force  - Force set migration version (VERSION=N)"
	@echo ""
	@echo "Testing:"
	@echo "  make test           - Run Go tests"

# Build Docker images
build:
	@echo "Building Docker images..."
	docker-compose build

# Start all services
up:
	@echo "Starting services..."
	docker-compose up -d
	@echo "Services started. API available at http://localhost:8080"

# Stop all services
down:
	@echo "Stopping services..."
	docker-compose down

# Restart all services (no rebuild)
restart: down up

# Rebuild images and restart services
rebuild:
	@echo "Rebuilding images and restarting services..."
	docker-compose up -d --build
	@echo "Rebuild complete. API available at http://localhost:8080"

# View logs from all services
logs:
	docker-compose logs -f

# View API logs
logs-api:
	docker-compose logs -f api

# View PostgreSQL logs
logs-db:
	docker-compose logs -f postgres

# View Redis logs
logs-redis:
	docker-compose logs -f redis

# Access API container shell
shell:
	docker-compose exec api sh

# Access PostgreSQL shell
psql:
	docker-compose exec postgres psql -U spotifyapp -d spotify_playlists

# Access Redis CLI
redis-cli:
	docker-compose exec redis redis-cli

# Stop services and remove volumes (clean slate)
clean:
	@echo "Stopping services and removing volumes..."
	docker-compose down -v
	@echo "Cleanup complete"

# Run migrations up
migrate-up:
	@echo "Running migrations..."
	docker-compose run --rm migrate -path /migrations -database "postgres://spotifyapp:dev_password_change_in_production@postgres:5432/spotify_playlists?sslmode=disable" up

# Rollback last migration
migrate-down:
	@echo "Rolling back last migration..."
	docker-compose run --rm migrate -path /migrations -database "postgres://spotifyapp:dev_password_change_in_production@postgres:5432/spotify_playlists?sslmode=disable" down 1

# Create new migration
migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME is required. Usage: make migrate-create NAME=migration_name"; \
		exit 1; \
	fi
	@echo "Creating migration: $(NAME)"
	@docker run --rm -v $(PWD)/migrations:/migrations migrate/migrate create -ext sql -dir /migrations -seq $(NAME)

# Force migration version
migrate-force:
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION is required. Usage: make migrate-force VERSION=N"; \
		exit 1; \
	fi
	@echo "Forcing migration version to $(VERSION)..."
	docker-compose run --rm migrate -path /migrations -database "postgres://spotifyapp:dev_password_change_in_production@postgres:5432/spotify_playlists?sslmode=disable" force $(VERSION)

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...
