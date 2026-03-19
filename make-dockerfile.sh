#!/bin/bash

# ============================================================================
# new-api Docker Hub 构建脚本 (QuantumNous)
# 使用 gum 美化输出（可选）
# ============================================================================

set -e

# 颜色定义（兼容不支持 gum 的情况）
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 检查 gum 是否安装
USE_GUM=false
if command -v gum &> /dev/null; then
    USE_GUM=true
fi

# 兼容函数：输出带颜色的消息
log_info() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 39 "$(gum style --bold '[INFO]') $1"
    else
        echo -e "${BLUE}[INFO]${NC} $1"
    fi
}

log_success() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 82 "$(gum style --bold '[SUCCESS]') $1"
    else
        echo -e "${GREEN}[SUCCESS]${NC} $1"
    fi
}

log_warn() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 214 "$(gum style --bold '[WARN]') $1"
    else
        echo -e "${YELLOW}[WARN]${NC} $1"
    fi
}

log_error() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 196 "$(gum style --bold '[ERROR]') $1"
    else
        echo -e "${RED}[ERROR]${NC} $1"
    fi
}

# 打印分隔线
print_separator() {
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 240 "────────────────────────────────────────────────────────────"
    else
        echo "────────────────────────────────────────────────────────────"
    fi
}

# 打印步骤标题
print_step() {
    local step_num=$1
    local step_title=$2
    echo ""
    if [ "$USE_GUM" = true ]; then
        gum style --foreground 39 --bold "[$step_num] $step_title"
    else
        echo -e "${BLUE}[$step_num] $step_title${NC}"
    fi
    print_separator
}

# 打印 ASCII Banner
print_banner() {
    if [ "$USE_GUM" = true ]; then
        gum style \
            --foreground 57 --bold \
            "  _  _       ___  ___  ___                   " \
            " | \\| |___ _/ _ \\/ __|/ __|                  " \
            " | .\` / -_) (_) \\__ \\ (__                   " \
            " |_|\\_\\___|\\___/|___/\\___|                  " \
            "                                             " \
            "        AI API Gateway / Proxy               " \
            "                                             " \
            "      Docker Hub Build System v1.1"
    else
        echo -e "${BLUE}  _  _       ___  ___  ___${NC}"
        echo -e "${BLUE} | \\| |___ _/ _ \\/ __|/ __|${NC}"
        echo -e "${BLUE} | .\` / -_) (_) \\__ \\ (__${NC}"
        echo -e "${BLUE} |_|\\_\\___|\\___/|___/\\___|${NC}"
        echo ""
        echo -e "${BLUE}       AI API Gateway / Proxy${NC}"
        echo ""
        echo -e "${BLUE}     Docker Hub Build System v1.1${NC}"
    fi
}

# 显示帮助信息
show_help() {
    print_banner
    echo ""
    log_info "使用方法: ./make-dockerfile.sh [命令] [参数]"
    echo ""
    echo "命令:"
    echo "  build     构建 Docker 镜像并推送到 Docker Hub"
    echo "  login     登录 Docker Hub（可选，如已登录可跳过）"
    echo "  help      显示此帮助信息"
    echo ""
    echo "build 命令参数:"
    echo "  ./make-dockerfile.sh build [username] [password|token] [version] [platform]"
    echo ""
    echo "  参数说明:"
    echo "    username       - Docker Hub 用户名（可选，如已登录可省略）"
    echo "    password|token - Docker Hub 密码或 Access Token（可选）"
    echo "    version        - 版本号（可选，不指定则自动从 VERSION 文件读取或递增）"
    echo "    platform       - 目标平台（可选，默认: linux/amd64）"
    echo "                     支持: linux/amd64, linux/arm64, linux/amd64,linux/arm64"
    echo ""
    echo "示例:"
    echo "  # 已登录 Docker Hub，直接构建"
    echo "  ./make-dockerfile.sh build"
    echo ""
    echo "  # 已登录，指定版本"
    echo "  ./make-dockerfile.sh build myuser '' 1.0.0"
    echo ""
    echo "  # 未登录，提供凭证"
    echo "  ./make-dockerfile.sh build myuser mytoken"
    echo ""
    echo "  # 指定版本和平台"
    echo "  ./make-dockerfile.sh build myuser mytoken 1.0.0 linux/arm64"
    echo ""
    echo "  # 多平台构建"
    echo "  ./make-dockerfile.sh build myuser mytoken 1.0.0 linux/amd64,linux/arm64"
    echo ""
    echo "提示:"
    echo "  - 如果已通过 'docker login' 登录，可省略用户名和密码"
    echo "  - 推荐使用 Docker Hub Access Token 而非密码"
    echo "  - Access Token 可在 https://hub.docker.com/settings/security 创建"
    echo ""
    exit 0
}

# 检查 Docker Hub 登录状态
check_docker_login() {
    # 检查是否有登录凭证
    if [ -f "$HOME/.docker/config.json" ]; then
        if grep -q "docker.io" "$HOME/.docker/config.json" 2>/dev/null || \
           grep -q "https://index.docker.io/v1/" "$HOME/.docker/config.json" 2>/dev/null; then
            return 0
        fi
    fi
    return 1
}

# 获取当前登录的用户名
get_logged_in_user() {
    # 方法1: 从 credential store 获取（Docker Desktop 方式）
    if [ -f "$HOME/.docker/config.json" ]; then
        # 检查是否使用 credsStore
        local creds_store
        creds_store=$(grep -o '"credsStore"[[:space:]]*:[[:space:]]*"[^"]*"' "$HOME/.docker/config.json" 2>/dev/null | cut -d'"' -f4)

        if [ -n "$creds_store" ]; then
            # 使用 credential helper 获取用户名
            local creds
            case "$creds_store" in
                desktop)
                    creds=$(docker-credential-desktop list 2>/dev/null)
                    ;;
                osxkeychain)
                    creds=$(docker-credential-osxkeychain list 2>/dev/null)
                    ;;
                *)
                    creds=$("$creds_store" list 2>/dev/null)
                    ;;
            esac

            if [ -n "$creds" ]; then
                # 从 JSON 中提取 Docker Hub 用户名
                echo "$creds" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    # Docker Hub 的 URL
    for key in ['https://index.docker.io/v1/', 'docker.io', 'https://registry-1.docker.io/v2/']:
        if key in data:
            print(data[key])
            break
except:
    pass
" 2>/dev/null
                return
            fi
        fi

        # 方法2: 从 config.json 的 auths 字段获取（传统方式）
        if grep -q '"https://index.docker.io/v1/"' "$HOME/.docker/config.json"; then
            python3 -c "
import sys, json
try:
    with open('$HOME/.docker/config.json', 'r') as f:
        data = json.load(f)
    auths = data.get('auths', {})
    hub_auth = auths.get('https://index.docker.io/v1/', {})
    # 如果有 auth 字段，解码获取用户名
    if 'auth' in hub_auth:
        import base64
        decoded = base64.b64decode(hub_auth['auth']).decode('utf-8')
        username = decoded.split(':')[0]
        print(username)
except:
    pass
" 2>/dev/null
            return
        fi
    fi

    # 方法3: 尝试从 docker info 获取
    docker info 2>/dev/null | grep -i "username" | awk '{print $2}'
}

# 登录 Docker Hub
login_docker_hub() {
    local username=$1
    local password=$2

    # 优先检查是否已登录（通过 docker login 命令）
    if check_docker_login; then
        local current_user
        current_user=$(get_logged_in_user)

        # 如果提供了用户名且与已登录用户不同，需要重新登录
        if [ -n "$username" ] && [ "$username" != "$current_user" ] && [ -n "$password" ]; then
            log_info "检测到已登录用户 ($current_user) 与指定用户 ($username) 不同，正在切换..."
            log_info "正在登录 Docker Hub..."
            log_info "用户名: $username"

            if echo "$password" | docker login -u "$username" --password-stdin 2>&1 | grep -q "Login Succeeded"; then
                log_success "Docker Hub 登录成功"
                return 0
            else
                log_error "Docker Hub 登录失败，请检查用户名和密码/Token"
                log_info "提示: 推荐使用 Access Token，可在 https://hub.docker.com/settings/security 创建"
                return 1
            fi
        fi

        # 使用已登录状态
        if [ -n "$current_user" ]; then
            log_success "检测到已登录 Docker Hub (用户: $current_user)"
        else
            log_success "检测到已登录 Docker Hub"
        fi
        return 0
    fi

    # 未登录状态，检查是否提供了凭证
    if [ -n "$username" ] && [ -n "$password" ]; then
        log_info "正在登录 Docker Hub..."
        log_info "用户名: $username"

        # 登录 Docker Hub（使用 stdin 避免密码泄露到命令历史）
        if echo "$password" | docker login -u "$username" --password-stdin 2>&1 | grep -q "Login Succeeded"; then
            log_success "Docker Hub 登录成功"
            return 0
        else
            log_error "Docker Hub 登录失败，请检查用户名和密码/Token"
            log_info "提示: 推荐使用 Access Token，可在 https://hub.docker.com/settings/security 创建"
            return 1
        fi
    else
        log_error "未登录 Docker Hub，请提供用户名和密码/Token"
        log_info "使用方法: ./make-dockerfile.sh build <username> <password|token>"
        log_info "或先运行: docker login"
        return 1
    fi
}

# 构建并推送镜像
build_and_push() {
    # 处理空字符串参数（Makefile 可能传递 '' 作为参数）
    local username=$1
    local password=$2
    local specified_version=$3
    local target_platform=${4:-"linux/amd64"}

    # 将空字符串转换为真正的空值
    if [ "$username" = "''" ] || [ "$username" = '""' ] || [ -z "$username" ]; then
        username=""
    fi
    if [ "$password" = "''" ] || [ "$password" = '""' ] || [ -z "$password" ]; then
        password=""
    fi
    if [ "$specified_version" = "''" ] || [ "$specified_version" = '""' ] || [ -z "$specified_version" ]; then
        specified_version=""
    fi
    if [ "$target_platform" = "''" ] || [ "$target_platform" = '""' ]; then
        target_platform="linux/amd64"
    fi

    print_banner
    echo ""

    # ============================================================================
    # STEP 1: 确定用户名
    # ============================================================================
    print_step "1/5" "确定用户名"

    # 获取项目根目录
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    cd "$SCRIPT_DIR"

    # 如果没有提供用户名，尝试从已登录状态获取
    if [ -z "$username" ]; then
        if check_docker_login; then
            username=$(get_logged_in_user)
            if [ -z "$username" ]; then
                log_error "无法从已登录状态获取用户名，请手动指定"
                log_info "使用方法: ./make-dockerfile.sh build <username> <password|token>"
                exit 1
            fi
            log_info "从已登录状态获取用户名: $(gum style --bold --foreground 214 "$username" 2>/dev/null || echo "$username")"
        else
            log_error "未登录 Docker Hub，请提供用户名和密码/Token"
            log_info "使用方法: ./make-dockerfile.sh build <username> <password|token>"
            exit 1
        fi
    fi

    # 配置
    REGISTRY="docker.io"
    IMAGE_NAME="newapi-fix"
    FULL_IMAGE_NAME="$username/$IMAGE_NAME"

    echo ""

    # ============================================================================
    # STEP 2: 确定版本号
    # ============================================================================
    print_step "2/5" "确定版本号"

    if [ -n "$specified_version" ]; then
        VERSION=$specified_version
        log_info "使用指定版本: $(gum style --bold --foreground 214 "$VERSION" 2>/dev/null || echo "$VERSION")"
    else
        # 从 VERSION 文件读取版本
        if [ -f "VERSION" ]; then
            FILE_VERSION=$(cat VERSION | tr -d '[:space:]')
            if [ -n "$FILE_VERSION" ]; then
                VERSION=$FILE_VERSION
                log_info "从 VERSION 文件读取版本: $(gum style --bold --foreground 214 "$VERSION" 2>/dev/null || echo "$VERSION")"
            else
                log_warn "VERSION 文件为空，尝试从 Docker Hub 获取最新版本..."
                VERSION=$(get_remote_version "$FULL_IMAGE_NAME")
            fi
        else
            log_warn "VERSION 文件不存在，尝试从 Docker Hub 获取最新版本..."
            VERSION=$(get_remote_version "$FULL_IMAGE_NAME")
        fi
    fi

    echo ""

    # ============================================================================
    # STEP 3: 环境检查
    # ============================================================================
    print_step "3/5" "环境检查"

    # 检查 Docker 是否安装
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装，请先安装 Docker"
        exit 1
    fi
    log_success "Docker 已安装"

    # 检查 Docker buildx 是否可用
    if ! docker buildx version &> /dev/null; then
        log_warn "Docker buildx 不可用，将使用传统构建方式"
        USE_BUILDX=false
    else
        log_success "Docker buildx 可用"
        USE_BUILDX=true
    fi

    # 检查 Dockerfile 是否存在
    if [ ! -f "Dockerfile" ]; then
        log_error "Dockerfile 不存在于项目根目录"
        exit 1
    fi
    log_success "Dockerfile 已找到"

    # 检查 go.mod 是否存在
    if [ ! -f "go.mod" ]; then
        log_error "go.mod 不存在，请确保在项目根目录运行"
        exit 1
    fi
    log_success "go.mod 已找到"

    # 检测系统信息
    OS=$(uname -s)
    ARCH=$(uname -m)
    log_info "检测到系统: $OS $ARCH"
    log_info "目标平台: $target_platform"

    # 准备构建
    if command -v swag &> /dev/null; then
        log_info "正在生成 Swagger 文档..."
        if swag init -g main.go --parseDependency 2>/dev/null; then
            log_success "Swagger 文档生成成功"
        else
            log_warn "Swagger 文档生成失败，继续构建..."
        fi
    else
        log_info "跳过 Swagger 文档生成（swag 未安装）"
    fi

    # 确保 VERSION 文件存在并包含当前版本
    echo "$VERSION" > VERSION
    log_info "已更新 VERSION 文件: $VERSION"

    echo ""

    # ============================================================================
    # STEP 4: Docker 登录
    # ============================================================================
    print_step "4/5" "Docker Hub 登录状态"

    if ! login_docker_hub "$username" "$password"; then
        exit 1
    fi

    echo ""

    # ============================================================================
    # STEP 5: 构建并推送 Docker 镜像
    # ============================================================================
    print_step "5/5" "构建并推送 Docker 镜像"

    VERSION_TAG="$FULL_IMAGE_NAME:$VERSION"
    LATEST_TAG="$FULL_IMAGE_NAME:latest"

    echo ""
    if [ "$USE_GUM" = true ]; then
        gum style \
            --border normal --border-foreground 57 --padding "0 2" \
            "$(gum style --bold --foreground 57 "构建配置")" \
            "" \
            "$(gum style --foreground 245 "镜像名称: $IMAGE_NAME")" \
            "$(gum style --foreground 245 "完整路径: $FULL_IMAGE_NAME")" \
            "$(gum style --foreground 245 "版本标签: $VERSION")" \
            "$(gum style --foreground 245 "目标平台: $target_platform")" \
            "$(gum style --foreground 245 "Docker Hub: https://hub.docker.com/r/$FULL_IMAGE_NAME")"
    else
        echo "┌────────────────────────────────────────────────────────┐"
        echo "│ 构建配置                                               │"
        echo "├────────────────────────────────────────────────────────┤"
        echo "│ 镜像名称: $IMAGE_NAME"
        echo "│ 完整路径: $FULL_IMAGE_NAME"
        echo "│ 版本标签: $VERSION"
        echo "│ 目标平台: $target_platform"
        echo "│ Docker Hub: https://hub.docker.com/r/$FULL_IMAGE_NAME"
        echo "└────────────────────────────────────────────────────────┘"
    fi
    echo ""

    # 构建函数
    build_image() {
        log_info "开始构建 Docker 镜像..."
        echo ""

        if [ "$USE_BUILDX" = true ]; then
            # 使用 buildx 构建多平台镜像并直接推送
            local build_cmd="docker buildx build \
                --platform $target_platform \
                -f Dockerfile \
                -t '$VERSION_TAG' \
                -t '$LATEST_TAG' \
                --push ."

            if eval "$build_cmd"; then
                return 0
            else
                return 1
            fi
        else
            # 传统构建方式
            log_warn "使用传统构建方式，仅构建当前平台镜像"

            # 构建镜像
            if docker build -f Dockerfile -t "$VERSION_TAG" -t "$LATEST_TAG" .; then
                log_success "镜像构建成功"

                # 推送版本标签
                log_info "推送版本标签: $VERSION_TAG"
                if docker push "$VERSION_TAG"; then
                    log_success "版本标签推送成功"
                else
                    log_error "版本标签推送失败"
                    return 1
                fi

                # 推送最新标签
                log_info "推送最新标签: $LATEST_TAG"
                if docker push "$LATEST_TAG"; then
                    log_success "最新标签推送成功"
                else
                    log_error "最新标签推送失败"
                    return 1
                fi

                return 0
            else
                return 1
            fi
        fi
    }

    # 执行构建
    if build_image; then
        echo ""
        echo ""
        if [ "$USE_GUM" = true ]; then
            gum style \
                --border double --border-foreground 82 \
                --padding "1 3" --margin "1 0" \
                "$(gum style --bold --foreground 82 '🎉 构建成功!')" \
                "" \
                "$(gum style --foreground 82 "镜像已成功推送到 Docker Hub")" \
                "" \
                "$(gum style --foreground 245 "版本标签:")" \
                "$(gum style --foreground 240 "  📦 $VERSION_TAG")" \
                "" \
                "$(gum style --foreground 245 "最新标签:")" \
                "$(gum style --foreground 240 "  🏷️  $LATEST_TAG")" \
                "" \
                "$(gum style --foreground 245 "Docker Hub 页面:")" \
                "$(gum style --foreground 240 "  🌐 https://hub.docker.com/r/$FULL_IMAGE_NAME")" \
                "" \
                "$(gum style --foreground 245 "拉取命令:")" \
                "$(gum style --foreground 240 "  docker pull $VERSION_TAG")" \
                "$(gum style --foreground 240 "  docker pull $LATEST_TAG")"
        else
            echo "╔════════════════════════════════════════════════════════╗"
            echo "║              🎉 构建成功!                              ║"
            echo "╠════════════════════════════════════════════════════════╣"
            echo "║ 镜像已成功推送到 Docker Hub                            ║"
            echo "║                                                        ║"
            echo "║ 版本标签:                                              ║"
            echo "║   📦 $VERSION_TAG"
            echo "║                                                        ║"
            echo "║ 最新标签:                                              ║"
            echo "║   🏷️  $LATEST_TAG"
            echo "║                                                        ║"
            echo "║ Docker Hub 页面:                                       ║"
            echo "║   🌐 https://hub.docker.com/r/$FULL_IMAGE_NAME"
            echo "║                                                        ║"
            echo "║ 拉取命令:                                              ║"
            echo "║   docker pull $VERSION_TAG"
            echo "║   docker pull $LATEST_TAG"
            echo "╚════════════════════════════════════════════════════════╝"
        fi
        echo ""
        log_success "所有操作已完成!"
        exit 0
    else
        echo ""
        echo ""
        if [ "$USE_GUM" = true ]; then
            gum style \
                --border double --border-foreground 196 \
                --padding "1 3" --margin "1 0" \
                "$(gum style --bold --foreground 196 '❌ 构建失败')" \
                "" \
                "$(gum style --foreground 196 "Docker 镜像构建失败，请检查错误信息")"
        else
            echo "╔════════════════════════════════════════════════════════╗"
            echo "║              ❌ 构建失败                               ║"
            echo "╠════════════════════════════════════════════════════════╣"
            echo "║ Docker 镜像构建失败，请检查错误信息                    ║"
            echo "╚════════════════════════════════════════════════════════╝"
        fi
        echo ""
        log_error "构建失败，请排查错误后重试"
        exit 1
    fi
}

# 从 Docker Hub 获取最新版本
get_remote_version() {
    local image_name=$1

    log_info "正在从 Docker Hub 获取最新版本..."

    # 使用 Docker Hub API 获取标签列表
    local tags_json
    tags_json=$(curl -s "https://registry.hub.docker.com/v2/repositories/$image_name/tags/" 2>/dev/null)

    if [ -z "$tags_json" ]; then
        log_warn "无法获取远程版本信息，使用初始版本: 1.0.0"
        echo "1.0.0"
        return
    fi

    # 解析版本标签（排除 latest 和非数字版本）
    local latest_version
    latest_version=$(echo "$tags_json" | \
        python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    tags = [r['name'] for r in data.get('results', [])]
    version_tags = [t for t in tags if t != 'latest' and t.replace('.', '').replace('-', '').isdigit()]
    if version_tags:
        # 按语义版本排序
        from packaging import version
        version_tags.sort(key=lambda x: version.parse(x), reverse=True)
        print(version_tags[0])
    else:
        print('')
except Exception as e:
    print('')
" 2>/dev/null || echo "")

    if [ -z "$latest_version" ]; then
        log_warn "未找到远程版本，使用初始版本: 1.0.0"
        echo "1.0.0"
    else
        # 递增补丁版本
        local major minor patch
        major=$(echo "$latest_version" | cut -d. -f1)
        minor=$(echo "$latest_version" | cut -d. -f2)
        patch=$(echo "$latest_version" | cut -d. -f3)

        patch=$((patch + 1))
        local new_version="$major.$minor.$patch"

        log_success "最新版本: $latest_version → 新版本: $(gum style --bold --foreground 214 "$new_version" 2>/dev/null || echo "$new_version")"
        echo "$new_version"
    fi
}

# 主入口
case "${1:-help}" in
    build)
        # 所有参数都是可选的
        build_and_push "$2" "$3" "$4" "$5"
        ;;
    login)
        if [ -z "$2" ]; then
            # 检查是否已登录
            if check_docker_login; then
                print_banner
                echo ""
                log_success "已登录 Docker Hub"
                current_user=$(get_logged_in_user)
                if [ -n "$current_user" ]; then
                    log_info "当前用户: $current_user"
                fi
                exit 0
            else
                log_error "未登录 Docker Hub，请提供用户名和密码/Token"
                echo ""
                echo "使用方法: ./make-dockerfile.sh login <username> <password|token>"
                exit 1
            fi
        fi
        if [ -z "$3" ]; then
            log_error "请提供密码或 Access Token"
            echo ""
            echo "使用方法: ./make-dockerfile.sh login <username> <password|token>"
            exit 1
        fi
        print_banner
        echo ""
        login_docker_hub "$2" "$3"
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        log_error "未知命令: $1"
        echo ""
        echo "可用命令: build, login, help"
        echo "运行 './make-dockerfile.sh help' 获取更多信息"
        exit 1
        ;;
esac
