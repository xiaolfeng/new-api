FRONTEND_DIR = ./web/default
FRONTEND_CLASSIC_DIR = ./web/classic
BACKEND_DIR = .
DEV_FRONTEND_DEFAULT_PORT ?= 5173
DEV_FRONTEND_CLASSIC_PORT ?= 5174
DEV_COMPOSE_FILE = docker-compose.dev.yml
DEV_POSTGRES_SERVICE = postgres
DEV_BACKEND_SERVICE = new-api
DEV_POSTGRES_DB = new-api
DEV_POSTGRES_USER = root
DEV_SQLITE_PATH ?= one-api.db

.PHONY: all build-frontend build-frontend-classic build-all-frontends start-backend dev dev-api dev-api-rebuild dev-web dev-web-classic docker-build docker-login docker-help reset-setup

all: build-all-frontends start-backend

build-frontend:
	@echo "Building default frontend..."
	@cd ./web && bun install --frozen-lockfile
	@cd $(FRONTEND_DIR) && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat ../../VERSION) bun run build

build-frontend-classic:
	@echo "Building classic frontend..."
	@cd ./web && bun install --frozen-lockfile
	@cd $(FRONTEND_CLASSIC_DIR) && VITE_REACT_APP_VERSION=$(cat ../../VERSION) bun run build

build-all-frontends: build-frontend build-frontend-classic

start-backend:
	@echo "Starting backend dev server..."
	@cd $(BACKEND_DIR) && go run main.go &

dev-api:
	@echo "Starting backend services (docker)..."
	@docker compose -f $(DEV_COMPOSE_FILE) up -d

dev-api-rebuild:
	@echo "Rebuilding and starting backend service (docker)..."
	@docker compose -f $(DEV_COMPOSE_FILE) up -d --build $(DEV_BACKEND_SERVICE)

dev-web:
	@echo "Starting both frontend dev servers..."
	@echo "Default frontend: http://localhost:$(DEV_FRONTEND_DEFAULT_PORT)"
	@echo "Classic frontend: http://localhost:$(DEV_FRONTEND_CLASSIC_PORT)"
	@cd ./web && bun install
	@(cd $(FRONTEND_DIR) && bun run dev -- --host 0.0.0.0 --port $(DEV_FRONTEND_DEFAULT_PORT)) & \
		default_pid=$$!; \
		(cd $(FRONTEND_CLASSIC_DIR) && bun run dev -- --host 0.0.0.0 --port $(DEV_FRONTEND_CLASSIC_PORT)) & \
		classic_pid=$$!; \
		trap 'kill $$default_pid $$classic_pid 2>/dev/null; wait $$default_pid $$classic_pid 2>/dev/null; exit 130' INT TERM; \
		while kill -0 $$default_pid 2>/dev/null && kill -0 $$classic_pid 2>/dev/null; do \
			sleep 1; \
		done; \
		if ! kill -0 $$default_pid 2>/dev/null; then \
			wait $$default_pid; \
			status=$$?; \
			kill $$classic_pid 2>/dev/null; \
			wait $$classic_pid 2>/dev/null; \
			exit $$status; \
		fi; \
		wait $$classic_pid; \
		status=$$?; \
		kill $$default_pid 2>/dev/null; \
		wait $$default_pid 2>/dev/null; \
		exit $$status

dev-web-classic:
	@echo "Starting classic frontend dev server..."
	@cd ./web && bun install
	@cd $(FRONTEND_CLASSIC_DIR) && bun run dev -- --host 0.0.0.0 --port $(DEV_FRONTEND_CLASSIC_PORT)

dev: dev-api dev-web

# ============================================================================
# Docker 构建命令 (推送到 Docker Hub)
# ============================================================================
# 使用方法:
#   # 已登录状态，直接构建
#   make docker-build
#
#   # 已登录状态，指定版本
#   make docker-build VERSION=1.0.0
#
#   # 未登录状态，提供凭证
#   make docker-build USER=<username> PASS=<password|token>
#
#   # 指定平台（支持多平台）
#   make docker-build PLATFORM=linux/amd64,linux/arm64
#
#   # 完整参数
#   make docker-build USER=<username> PASS=<password|token> VERSION=<version> PLATFORM=<platform>
#
# 示例:
#   make docker-build
#   make docker-build VERSION=1.0.0
#   make docker-build USER=myuser PASS=mytoken
#   make docker-build USER=myuser PASS=mytoken VERSION=1.0.0
#   make docker-build USER=myuser PASS=mytoken VERSION=1.0.0 PLATFORM=linux/arm64
#   make docker-build PLATFORM=linux/amd64,linux/arm64
# ============================================================================

docker-build:
	@./make-dockerfile.sh build $(USER) $(PASS) $(VERSION) $(PLATFORM)

docker-login:
ifdef USER
ifdef PASS
	@./make-dockerfile.sh login $(USER) $(PASS)
else
	@./make-dockerfile.sh login
endif
else
	@./make-dockerfile.sh login
endif

docker-help:
	@./make-dockerfile.sh help

reset-setup:
	@echo "Resetting local setup wizard state..."
	@if docker compose -f $(DEV_COMPOSE_FILE) ps --services --status running | grep -qx "$(DEV_POSTGRES_SERVICE)"; then \
		echo "Detected running docker dev PostgreSQL. Removing setup record and root users..."; \
		docker compose -f $(DEV_COMPOSE_FILE) exec -T $(DEV_POSTGRES_SERVICE) \
			psql -U $(DEV_POSTGRES_USER) -d $(DEV_POSTGRES_DB) \
			-c 'DELETE FROM setups;' \
			-c 'DELETE FROM users WHERE role = 100;' \
			-c "DELETE FROM options WHERE key IN ('SelfUseModeEnabled', 'DemoSiteEnabled');"; \
		echo "Restarting docker dev backend so setup status is recalculated..."; \
		docker compose -f $(DEV_COMPOSE_FILE) restart $(DEV_BACKEND_SERVICE); \
	elif db_path="$${SQLITE_PATH:-$(DEV_SQLITE_PATH)}"; db_path="$${db_path%%\?*}"; [ -f "$$db_path" ]; then \
		db_path="$${SQLITE_PATH:-$(DEV_SQLITE_PATH)}"; \
		db_path="$${db_path%%\?*}"; \
		echo "Detected local SQLite database: $$db_path"; \
		sqlite3 "$$db_path" \
			"DELETE FROM setups; DELETE FROM users WHERE role = 100; DELETE FROM options WHERE key IN ('SelfUseModeEnabled', 'DemoSiteEnabled');"; \
		echo "SQLite setup state reset. Restart the local backend process before testing the setup wizard."; \
	else \
		echo "No running docker dev PostgreSQL or local SQLite database found."; \
		echo "Start the dev stack with 'make dev-api', or set SQLITE_PATH/DEV_SQLITE_PATH to your local SQLite database."; \
		exit 1; \
	fi
