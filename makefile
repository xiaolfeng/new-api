FRONTEND_DIR = ./web
BACKEND_DIR = .

.PHONY: all build-frontend start-backend docker-build docker-login docker-help

all: build-frontend start-backend

build-frontend:
	@echo "Building frontend..."
	@cd $(FRONTEND_DIR) && bun install && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) bun run build

start-backend:
	@echo "Starting backend dev server..."
	@cd $(BACKEND_DIR) && go run main.go &

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


