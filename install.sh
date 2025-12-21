#!/bin/bash

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${CYAN}ℹ${NC} ${BLUE}$1${NC}"
}

print_success() {
    echo -e "${GREEN}✓${NC} ${GREEN}$1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} ${YELLOW}$1${NC}"
}

print_error() {
    echo -e "${RED}✗${NC} ${RED}$1${NC}"
}

print_step() {
    echo -e "\n${BOLD}${MAGENTA}▶${NC} ${BOLD}$1${NC}"
}

print_separator() {
    echo -e "${CYAN}────────────────────────────────────────────────────────${NC}"
}

# 检查命令是否存在
check_command() {
    if ! command -v "$1" &> /dev/null; then
        print_error "$1 命令未找到，请先安装 $1"
        exit 1
    fi
}

# 检查必要的命令
check_command "curl"
check_command "jq"
check_command "sha256sum"

# 检测 Linux 发行版
detect_linux_distro() {
    if [ ! -f /etc/os-release ]; then
        print_warning "无法检测 Linux 发行版（/etc/os-release 不存在）"
        return 1
    fi

    # 读取发行版信息
    . /etc/os-release

    DISTRO_ID=$(echo "$ID" | tr '[:upper:]' '[:lower:]')
    DISTRO_ID_LIKE=$(echo "$ID_LIKE" | tr '[:upper:]' '[:lower:]')

    # 标准化发行版名称
    case "$DISTRO_ID" in
        ubuntu|debian)
            LINUX_DISTRO="$DISTRO_ID"
            PACKAGE_MANAGER="apt"
            ;;
        arch|archlinux|manjaro)
            LINUX_DISTRO="arch"
            PACKAGE_MANAGER="pacman"
            ;;
        centos|rhel|rocky|almalinux)
            LINUX_DISTRO="rhel"
            # 检测使用 yum 还是 dnf
            if command -v dnf &> /dev/null; then
                PACKAGE_MANAGER="dnf"
            else
                PACKAGE_MANAGER="yum"
            fi
            ;;
        fedora)
            LINUX_DISTRO="fedora"
            PACKAGE_MANAGER="dnf"
            ;;
        opensuse*|sles)
            LINUX_DISTRO="suse"
            PACKAGE_MANAGER="zypper"
            ;;
        *)
            # 尝试从 ID_LIKE 推断
            if echo "$DISTRO_ID_LIKE" | grep -q "debian\|ubuntu"; then
                LINUX_DISTRO="debian"
                PACKAGE_MANAGER="apt"
            elif echo "$DISTRO_ID_LIKE" | grep -q "rhel\|fedora\|centos"; then
                LINUX_DISTRO="rhel"
                if command -v dnf &> /dev/null; then
                    PACKAGE_MANAGER="dnf"
                else
                    PACKAGE_MANAGER="yum"
                fi
            elif echo "$DISTRO_ID_LIKE" | grep -q "arch"; then
                LINUX_DISTRO="arch"
                PACKAGE_MANAGER="pacman"
            else
                print_warning "未识别的 Linux 发行版: $DISTRO_ID"
                return 1
            fi
            ;;
    esac

    print_info "检测到 Linux 发行版: ${BOLD}$LINUX_DISTRO${NC} (包管理器: $PACKAGE_MANAGER)"
    return 0
}

# 检查 systemd 是否已安装
check_systemd() {
    # 检查 systemctl 命令是否存在
    if command -v systemctl &> /dev/null; then
        # 检查 systemd 目录是否存在
        if [ -d /usr/lib/systemd ] || [ -d /lib/systemd ]; then
            # 检查 systemd 是否正在运行
            if systemctl --version &> /dev/null; then
                print_success "systemd 已安装并可用"
                return 0
            fi
        fi
    fi

    print_warning "systemd 未安装或不可用"
    return 1
}

# 安装 systemd
install_systemd() {
    [ -z "$LINUX_DISTRO" ] || [ -z "$PACKAGE_MANAGER" ] && print_error "无法确定 Linux 发行版，无法自动安装 systemd" && return 1

    if [ "$EUID" -ne 0 ]; then
        print_error "安装 systemd 需要 root 权限"
        print_info "请使用 sudo 运行此脚本，或手动安装 systemd："
        local install_cmd
        case "$PACKAGE_MANAGER" in
            apt) install_cmd="sudo apt-get update && sudo apt-get install -y systemd" ;;
            pacman) install_cmd="sudo pacman -S --noconfirm systemd" ;;
            dnf|yum) install_cmd="sudo $PACKAGE_MANAGER install -y systemd" ;;
            zypper) install_cmd="sudo zypper install -y systemd" ;;
        esac
        echo -e "  ${CYAN}$install_cmd${NC}"
        return 1
    fi

    print_info "正在安装 systemd..."

    case "$PACKAGE_MANAGER" in
        apt)
            apt-get update && apt-get install -y systemd systemd-sysv && print_success "systemd 安装成功" && return 0
            ;;
        pacman)
            pacman -S --noconfirm systemd && print_success "systemd 安装成功" && return 0
            ;;
        dnf|yum)
            $PACKAGE_MANAGER install -y systemd && print_success "systemd 安装成功" && return 0
            ;;
        zypper)
            zypper install -y systemd && print_success "systemd 安装成功" && return 0
            ;;
        *)
            print_error "不支持的包管理器: $PACKAGE_MANAGER"
            return 1
            ;;
    esac

    print_error "systemd 安装失败"
    return 1
}

# 检查并安装 systemd
check_and_install_systemd() {
    # 检测 Linux 发行版
    if ! detect_linux_distro; then
        print_warning "无法检测 Linux 发行版，跳过 systemd 检查"
        return 0
    fi

    # 检查 systemd 是否已安装
    if check_systemd; then
        return 0
    fi

    # 如果未安装，尝试安装
    print_info "systemd 未安装，尝试自动安装..."
    if install_systemd; then
        sleep 1
        if check_systemd; then
            return 0
        else
            print_warning "systemd 安装后仍不可用，可能需要重启系统"
            print_warning "脚本将继续执行，但建议手动安装 systemd 以确保服务管理功能正常"
            return 0
        fi
    else
        print_warning "systemd 自动安装失败，请手动安装"
        print_warning "脚本将继续执行，但建议手动安装 systemd 以确保服务管理功能正常"
        return 0
    fi
}

# 获取系统信息
get_system_info() {
    OS_TYPE="linux"
    ARCH=$(uname -m)

    # 标准化架构名称
    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7l|arm) ARCH="arm" ;;
        i386|i686) ARCH="386" ;;
        *)
            print_error "不支持的架构: $ARCH"
            exit 1
            ;;
    esac

    print_info "系统类型: ${BOLD}$OS_TYPE-$ARCH${NC}"
}

# 获取最新版本信息
get_latest_version() {
    print_info "正在获取最新版本信息..."

    # 默认且仅使用 GitHub
    API_URL="https://api.github.com/repos/YunTower/CloudSentinel/releases/latest"
    RESPONSE=$(curl -s -H "Accept: application/vnd.github.v3+json" "$API_URL")

    if [ $? -ne 0 ]; then
        print_error "获取版本信息失败"
        exit 1
    fi

    # 检查响应是否包含错误
    if echo "$RESPONSE" | jq -e '.message' > /dev/null 2>&1; then
        ERROR_MSG=$(echo "$RESPONSE" | jq -r '.message')
        print_error "API 返回错误: $ERROR_MSG"
        exit 1
    fi

    TAG_NAME=$(echo "$RESPONSE" | jq -r '.tag_name // empty')
    if [ -z "$TAG_NAME" ] || [ "$TAG_NAME" = "null" ]; then
        print_error "无法获取版本标签"
        exit 1
    fi

    # 移除版本号前的 'v' 前缀
    TAG_NAME=${TAG_NAME#v}

    print_success "最新版本: ${BOLD}$TAG_NAME${NC}"

    # 保存 assets 信息
    ASSETS=$(echo "$RESPONSE" | jq -c '.assets // []')
}

# 查找匹配的二进制包
find_asset() {
    local expected_name=$1
    local asset_name=""
    local download_url=""

    # 遍历 assets 查找匹配的文件
    for asset in $(echo "$ASSETS" | jq -c '.[]'); do
        name=$(echo "$asset" | jq -r '.name // empty')

        if [ "$name" = "$expected_name" ]; then
            asset_name="$name"
            # GitHub 使用 browser_download_url
            download_url=$(echo "$asset" | jq -r '.browser_download_url // empty')

            if [ -n "$download_url" ] && [ "$download_url" != "null" ]; then
                echo "$asset_name|$download_url"
                return 0
            fi
        fi
    done

    return 1
}

# 下载文件
download_file() {
    local url=$1
    local output=$2
    local description=$3

    print_info "正在下载 $description..."
    if curl -L --progress-bar -o "$output" "$url"; then
        local file_size=$(du -h "$output" | cut -f1)
        print_success "$description 下载完成 (大小: $file_size)"
        return 0
    else
        print_error "$description 下载失败"
        return 1
    fi
}

# 计算文件的 SHA256
calculate_sha256() {
    local file=$1
    sha256sum "$file" | awk '{print $1}' | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]'
}

# 读取 SHA256 文件内容
read_sha256_file() {
    local file=$1
    head -n1 "$file" | awk '{print $1}' | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]'
}

# 校验文件完整性
verify_file() {
    local file=$1
    local sha256_file=$2

    print_info "正在校验文件完整性..."

    # 读取期望的哈希值
    local expected_hash=$(read_sha256_file "$sha256_file")
    # 计算实际的哈希值
    local actual_hash=$(calculate_sha256 "$file")

    # 再次确保去除所有空白字符
    expected_hash=$(printf '%s' "$expected_hash" | tr -d '[:space:]')
    actual_hash=$(printf '%s' "$actual_hash" | tr -d '[:space:]')

    # 比较哈希值
    if [ "$expected_hash" = "$actual_hash" ]; then
        print_success "文件校验通过"
        return 0
    else
        print_error "文件校验失败"
        print_error "期望 (长度 ${#expected_hash}): $expected_hash"
        print_error "实际 (长度 ${#actual_hash}): $actual_hash"
        # 使用 od 命令显示十六进制，帮助调试隐藏字符
        if command -v od &> /dev/null; then
            print_info "期望值十六进制: $(printf '%s' "$expected_hash" | od -An -tx1 | tr -d ' \n')"
            print_info "实际值十六进制: $(printf '%s' "$actual_hash" | od -An -tx1 | tr -d ' \n')"
        fi
        return 1
    fi
}

# 解压 tar.gz 文件
extract_tar_gz() {
    local archive=$1
    local dest_dir=$2

    print_info "正在解压文件..."

    if [ ! -d "$dest_dir" ]; then
        mkdir -p "$dest_dir"
    fi

    if tar -xzf "$archive" -C "$dest_dir"; then
        print_success "解压完成"
        return 0
    else
        print_error "解压失败"
        return 1
    fi
}

# 检查端口是否可用
is_port_available() {
    local port=$1
    for cmd in lsof netstat ss; do
        if command -v "$cmd" &> /dev/null; then
            case "$cmd" in
                lsof) lsof -i ":$port" &> /dev/null && return 1 ;;
                netstat) netstat -ln 2>/dev/null | grep -qE "[:.]$port " && return 1 ;;
                ss) ss -ln 2>/dev/null | grep -qE "[:.]$port " && return 1 ;;
            esac
            return 0
        fi
    done
    return 0  # 默认认为可用
}

# 获取公网IP地址
get_public_ip() {
    local public_ip=""
    
    # 尝试多个公网IP查询服务
    local ip_services=(
        "https://api.ip.sb/ip"
        "https://api.ipify.org"
        "https://icanhazip.com"
        "https://ipinfo.io/ip"
    )
    
    for service in "${ip_services[@]}"; do
        if command -v curl &> /dev/null; then
            public_ip=$(curl -s --max-time 3 "$service" 2>/dev/null | grep -oE "([0-9]{1,3}\.){3}[0-9]{1,3}" | head -n1)
            if [ -n "$public_ip" ] && [ "$public_ip" != "127.0.0.1" ]; then
                echo "$public_ip"
                return 0
            fi
        fi
    done
    
    return 1
}

# 获取所有可用的IP地址
get_all_ips() {
    local ips=()
    local ip

    # 提取 IP 的通用函数
    extract_ips() {
        local cmd_output=$1
        while IFS= read -r line; do
            ip=$(echo "$line" | grep -oE "inet ([0-9]{1,3}\.){3}[0-9]{1,3}" | awk '{print $2}' | grep -v "^127\.")
            [ -n "$ip" ] && ips+=("$ip")
        done <<< "$cmd_output"
    }

    # 按优先级尝试不同命令
    for cmd in "ip addr show" "ifconfig" "hostname -I"; do
        if command -v $(echo "$cmd" | awk '{print $1}') &> /dev/null; then
            if [ "$cmd" = "hostname -I" ]; then
                ip=$(eval "$cmd" 2>/dev/null | awk '{print $1}' | grep -v "^127\.")
                [ -n "$ip" ] && ips+=("$ip")
            else
                extract_ips "$(eval "$cmd" 2>/dev/null | grep "inet " || echo "")"
            fi
            [ ${#ips[@]} -gt 0 ] && break
        fi
    done

    # 去重并排序
    if [ ${#ips[@]} -gt 0 ]; then
        printf '%s\n' "${ips[@]}" | sort -u -t. -k1,1n -k2,2n -k3,3n -k4,4n
    else
        echo "127.0.0.1"
    fi
}

# 生成随机高位端口（8000-65535）
generate_random_port() {
    local min_port=8000
    local max_port=65535
    local port
    local max_attempts=100
    local attempt=0

    # 尝试最多 100 次
    while [ $attempt -lt $max_attempts ]; do
        # 生成随机端口
        if [ -n "$RANDOM" ]; then
            port=$((RANDOM % (max_port - min_port + 1) + min_port))
        else
            # 如果没有 RANDOM，使用时间戳
            port=$((8000 + (attempt * 17) % (max_port - min_port + 1)))
        fi

        # 检查端口是否可用
        if is_port_available "$port"; then
            echo "$port"
            return 0
        fi

        attempt=$((attempt + 1))
    done

    # 如果都不可用，返回默认端口
    print_warning "无法找到可用端口，使用默认端口 8000"
    echo "8000"
}

# 创建 cloudsentinel 用户
create_cloudsentinel_user() {
    local cloudsentinel_dir=$1

    # 检查是否有 root 权限
    if [ "$EUID" -ne 0 ]; then
        print_error "创建用户需要 root 权限，请使用 sudo 运行此脚本"
        return 1
    fi

    # 检查用户是否已存在
    if id "cloudsentinel" &>/dev/null; then
        print_info "用户 cloudsentinel 已存在"
    else
        print_info "正在创建 cloudsentinel 用户..."
        # 创建系统用户（无登录 shell，无主目录）
        if useradd -r -s /bin/false cloudsentinel 2>/dev/null; then
            print_success "用户 cloudsentinel 创建成功"
        else
            print_error "创建用户失败"
            return 1
        fi
    fi

    # 设置目录所有权（同时授予 root 组访问权限）
    print_info "正在设置目录权限..."
    if chown -R cloudsentinel:root "$cloudsentinel_dir"; then
        print_success "目录权限设置成功"
    else
        print_error "设置目录权限失败"
        return 1
    fi

    # 设置目录权限：
    # - 2770: owner/group 可读写执行，其他无权限；setgid 确保新文件继承 root 组
    chmod 2770 "$cloudsentinel_dir"

    return 0
}

# 确保 cloudsentinel 用户可以进入安装目录（修复父目录不可 traverse 导致的 cd 失败）
ensure_cloudsentinel_can_access_dir() {
    local base_dir=$1
    local target_dir=$2

    # 仅在 root 且 cloudsentinel 用户存在时处理
    if [ "$EUID" -ne 0 ] || ! id "cloudsentinel" &>/dev/null; then
        return 0
    fi

    # 若已可进入则直接返回
    if sudo -u cloudsentinel test -d "$target_dir" 2>/dev/null && sudo -u cloudsentinel test -x "$target_dir" 2>/dev/null; then
        return 0
    fi

    print_warning "cloudsentinel 用户无法进入安装目录，尝试修复父目录权限（最小化授权）..."

    # 优先使用 ACL（更安全：不需要放开整个 base_dir 的访问权限）
    if command -v setfacl &>/dev/null; then
        # 允许 cloudsentinel traverse base_dir，允许其访问 target_dir
        setfacl -m "u:cloudsentinel:--x" "$base_dir" || true
        setfacl -m "u:cloudsentinel:rwx" "$target_dir" || true
        # 让 target_dir 下新建文件默认也给 cloudsentinel rwx（避免后续写入问题）
        setfacl -d -m "u:cloudsentinel:rwx" "$target_dir" || true
    else
        print_warning "未检测到 setfacl，将退化为 chmod o+x 方式放开父目录可进入权限"
        chmod o+x "$base_dir" || true
    fi

    # 再次校验
    if ! sudo -u cloudsentinel test -d "$target_dir" 2>/dev/null || ! sudo -u cloudsentinel test -x "$target_dir" 2>/dev/null; then
        print_error "仍无法让 cloudsentinel 进入安装目录：$target_dir"
        print_error "建议将安装目录选在 /opt 或 /srv 等公共路径，例如：/opt/cloudsentinel"
        return 1
    fi

    return 0
}

# 检查是否为 root 且 cloudsentinel 用户存在
is_root_with_cloudsentinel() {
    [ "$EUID" -eq 0 ] && id "cloudsentinel" &>/dev/null
}

# 显示手动执行命令提示
show_manual_command() {
    local cmd=$1
    if id "cloudsentinel" &>/dev/null; then
        echo -e "  ${CYAN}sudo -u cloudsentinel $INSTALL_DIR/dashboard $cmd${NC}"
    else
        echo -e "  ${CYAN}cd $INSTALL_DIR${NC}"
        echo -e "  ${CYAN}./dashboard $cmd${NC}"
    fi
}

# 以 cloudsentinel 用户执行命令
run_as_cloudsentinel() {
    local command=$1
    local working_dir=$2

    if is_root_with_cloudsentinel; then
        if [ -n "$working_dir" ]; then
            sudo -u cloudsentinel sh -c "cd '$working_dir' && $command"
        else
            sudo -u cloudsentinel sh -c "$command"
        fi
    else
        if [ -n "$working_dir" ]; then
            (cd "$working_dir" && eval "$command")
        else
            eval "$command"
        fi
    fi
}

# 准备执行环境
prepare_exec_env() {
    local binary_path=$1
    local install_dir=$2
    local original_dir=$(pwd)

    # 验证参数
    [ -z "$binary_path" ] || [ -z "$install_dir" ] && return 1
    [ ! -f "$binary_path" ] && return 1

    # 转换为绝对路径
    binary_path=$(cd "$(dirname "$binary_path")" && pwd)/$(basename "$binary_path")
    install_dir=$(cd "$install_dir" && pwd)

    # 切换到安装目录
    cd "$install_dir" || return 1
    [ ! -f ".env" ] && cd "$original_dir" && return 1

    # 临时修改 APP_ENV 为 local
    local original_app_env=$(grep "^APP_ENV=" .env | cut -d'=' -f2 || echo "production")
    if [ "$original_app_env" = "production" ]; then
        grep -q "^APP_ENV=" .env && sed -i.bak 's/^APP_ENV=.*/APP_ENV=local/' .env || echo "APP_ENV=local" >> .env
    fi

    # 返回信息
    _PREP_BINARY_PATH="$binary_path"
    _PREP_INSTALL_DIR="$install_dir"
    _PREP_ORIGINAL_DIR="$original_dir"
    _PREP_ORIGINAL_APP_ENV="$original_app_env"
    _PREP_HAS_BACKUP=$([ "$original_app_env" = "production" ] && echo "true" || echo "false")

    return 0
}

# 恢复执行环境
restore_exec_env() {
    if [ "$_PREP_HAS_BACKUP" = "true" ] && [ -f ".env.bak" ]; then
        mv .env.bak .env
    fi
    cd "$_PREP_ORIGINAL_DIR" || true
}

# 生成 .env 配置文件
generate_env_file() {
    local install_dir=$1
    local port=$2
    local env_file="$install_dir/.env"

    print_info "正在生成配置文件..."

    # 生成 .env 文件内容
    cat > "$env_file" << EOF
APP_NAME=CloudSentinel
APP_ENV=production
APP_DEBUG=false
APP_KEY=
APP_URL=http://0.0.0.0:$port
APP_HOST=0.0.0.0
APP_PORT=$port

JWT_SECRET=

LOG_CHANNEL=stack
LOG_LEVEL=debug

DB_CONNECTION=sqlite
DB_DATABASE=database.db

SESSION_DRIVER=file
SESSION_LIFETIME=120
EOF

    print_success "配置文件已生成: $env_file"
}

# 验证并保存密钥值
verify_secret() {
    local key_name=$1
    local min_length=${2:-32}
    local value=$(grep "^$key_name=" .env | cut -d'=' -f2- | tr -d '[:space:]')
    [ -n "$value" ] && [ ${#value} -ge $min_length ]
}

# 初始化程序
init_app() {
    local binary_path=$1
    local install_dir=$2

    if ! prepare_exec_env "$binary_path" "$install_dir"; then
        print_error "init_app 初始化环境失败"
        return 1
    fi

    print_info "正在初始化应用配置..."

    # 生成 APP_KEY
    print_info "正在生成 APP_KEY..."
    local key_output=$(run_as_cloudsentinel "\"$_PREP_BINARY_PATH\" key:generate" "$_PREP_INSTALL_DIR" 2>&1)
    if [ $? -eq 0 ]; then
        sleep 0.5
        if verify_secret "APP_KEY"; then
            print_success "APP_KEY 生成成功"
        else
            print_warning "APP_KEY 可能未正确写入"
            [ -n "$key_output" ] && echo -e "  ${YELLOW}$key_output${NC}" | head -n 3
        fi
    else
        print_error "APP_KEY 生成失败"
        [ -n "$key_output" ] && echo -e "  ${RED}$key_output${NC}" | head -n 3
        restore_exec_env
        return 1
    fi

    # 生成 JWT_SECRET
    print_info "正在生成 JWT_SECRET..."
    local jwt_output=$(run_as_cloudsentinel "\"$_PREP_BINARY_PATH\" jwt:secret" "$_PREP_INSTALL_DIR" 2>&1)
    if [ $? -eq 0 ]; then
        sleep 0.5
        if verify_secret "JWT_SECRET"; then
            print_success "JWT_SECRET 生成成功"
        else
            print_warning "JWT_SECRET 可能未正确写入"
            [ -n "$jwt_output" ] && echo -e "  ${YELLOW}$jwt_output${NC}" | head -n 3
        fi
    else
        print_error "JWT_SECRET 生成失败"
        [ -n "$jwt_output" ] && echo -e "  ${RED}$jwt_output${NC}" | head -n 3
        restore_exec_env
        return 1
    fi

    # 恢复 APP_ENV 为原始值（保留生成的密钥）
    if [ "$_PREP_HAS_BACKUP" = "true" ] && [ -f ".env.bak" ]; then
        local app_key_value=$(grep "^APP_KEY=" .env | cut -d'=' -f2-)
        local jwt_secret_value=$(grep "^JWT_SECRET=" .env | cut -d'=' -f2-)
        mv .env.bak .env
        sed -i 's/^APP_ENV=.*/APP_ENV='"$_PREP_ORIGINAL_APP_ENV"'/' .env
        [ -n "$app_key_value" ] && (grep -q "^APP_KEY=" .env && sed -i "s|^APP_KEY=.*|APP_KEY=$app_key_value|" .env || echo "APP_KEY=$app_key_value" >> .env)
        [ -n "$jwt_secret_value" ] && (grep -q "^JWT_SECRET=" .env && sed -i "s|^JWT_SECRET=.*|JWT_SECRET=$jwt_secret_value|" .env || echo "JWT_SECRET=$jwt_secret_value" >> .env)
        rm -f .env.bak
    else
        sed -i 's/^APP_ENV=.*/APP_ENV='"$_PREP_ORIGINAL_APP_ENV"'/' .env
    fi

    restore_exec_env
    print_success "应用配置初始化完成"
}

# 执行数据库迁移
run_migration() {
    local binary_path=$1
    local install_dir=$2

    if ! prepare_exec_env "$binary_path" "$install_dir"; then
        print_error "run_migration 初始化环境失败"
        return 1
    fi

    print_info "正在执行数据库迁移..."
    local migrate_output=$(run_as_cloudsentinel "\"$_PREP_BINARY_PATH\" migrate" "$_PREP_INSTALL_DIR" 2>&1)
    local migrate_exit_code=$?

    restore_exec_env

    if [ $migrate_exit_code -eq 0 ]; then
        print_success "数据库迁移完成"
        [ -n "$migrate_output" ] && echo "$migrate_output" | grep -qi "migrated\|migration" && echo -e "  ${CYAN}$migrate_output${NC}" | grep -i "migrated\|migration" | head -n 5
        return 0
    else
        print_error "数据库迁移失败"
        [ -n "$migrate_output" ] && echo -e "  ${RED}$migrate_output${NC}" | head -n 5
        return 1
    fi
}

# 生成管理员账号
generate_admin_account() {
    local binary_path=$1
    local install_dir=$2

    if ! prepare_exec_env "$binary_path" "$install_dir"; then
        print_error "generate_admin_account 初始化环境失败"
        return 1
    fi

    print_info "正在生成随机管理员账号..."
    local admin_output=$(run_as_cloudsentinel "\"$_PREP_BINARY_PATH\" generate:admin" "$_PREP_INSTALL_DIR" 2>&1)
    local admin_exit_code=$?

    restore_exec_env

    if [ $admin_exit_code -eq 0 ]; then
        print_success "管理员账号生成完成"
        
        # 优先使用专门的机器可读行提取
        local creds_line=$(echo "$admin_output" | grep "ADMIN_CREDENTIALS|" | head -n1)
        if [ -n "$creds_line" ]; then
            # 提取 ADMIN_CREDENTIALS|username|password 格式
            # 使用 cut 命令更可靠
            ADMIN_USERNAME=$(echo "$creds_line" | cut -d'|' -f2 | tr -d '[:space:]')
            ADMIN_PASSWORD=$(echo "$creds_line" | cut -d'|' -f3 | tr -d '[:space:]')
        fi

        # 兜底提取逻辑：从格式化的输出中提取
        if [ -z "$ADMIN_USERNAME" ] || [ -z "$ADMIN_PASSWORD" ]; then
            # 移除 ANSI 颜色代码后提取
            local clean_output=$(echo "$admin_output" | sed 's/\x1b\[[0-9;]*m//g')
            
            # 尝试提取用户名
            if [ -z "$ADMIN_USERNAME" ]; then
                # 使用更简单的提取方式
                ADMIN_USERNAME=$(echo "$clean_output" | grep -iE "(用户名|username)" | sed -n 's/.*[用户名username][:：]\s*\([a-zA-Z0-9]\{10\}\).*/\1/p' | head -n1 | tr -d '[:space:]')
            fi
            
            # 尝试提取密码
            if [ -z "$ADMIN_PASSWORD" ]; then
                # 使用更简单的提取方式，避免字符类中的特殊字符问题
                ADMIN_PASSWORD=$(echo "$clean_output" | grep -iE "(密码|password)" | sed -n 's/.*[密码password][:：]\s*\([^[:space:]]\{20,\}\).*/\1/p' | head -n1 | tr -d '[:space:]')
            fi
        fi
        
        # 调试输出
        if [ -z "$ADMIN_USERNAME" ] || [ -z "$ADMIN_PASSWORD" ]; then
            print_warning "无法从输出中提取管理员账号信息"
            
            # 检查是否是命令未定义的错误
            if echo "$admin_output" | grep -qi "not defined\|not found\|未定义"; then
                print_error "命令 generate:admin 未定义或不可用"
                print_info "这可能是因为："
                print_info "1. 二进制文件版本过旧，不包含此命令"
                print_info "2. 命令未正确注册"
                print_info "请手动检查二进制文件是否支持此命令："
                echo -e "  ${CYAN}cd $install_dir && ./dashboard list${NC}"
            else
                print_info "原始输出（前10行）："
                echo "$admin_output" | head -n 10 | sed 's/^/  /'
            fi
        fi
        
        return 0
    else
        print_error "管理员账号生成失败"
        [ -n "$admin_output" ] && echo -e "  ${RED}$admin_output${NC}" | head -n 5
        return 1
    fi
}

# 启动服务
start_service() {
    local binary_path=$1
    local install_dir=$2
    local port=$3

    if [ -z "$binary_path" ] || [ -z "$install_dir" ] || [ -z "$port" ]; then
        print_error "start_service 函数需要二进制文件路径、安装目录和端口参数"
        return 1
    fi

    # 确保使用绝对路径
    if [ ! -f "$binary_path" ]; then
        print_error "二进制文件不存在: $binary_path"
        return 1
    fi

    # 转换为绝对路径
    binary_path=$(cd "$(dirname "$binary_path")" && pwd)/$(basename "$binary_path")
    install_dir=$(cd "$install_dir" && pwd)

    print_info "正在启动服务（后台运行）..."
    print_info "服务将在后台运行，端口: ${BOLD}$port${NC}"

    # 保存当前目录
    local original_dir=$(pwd)

    # 切换到安装目录
    cd "$install_dir" || {
        print_error "无法切换到安装目录: $install_dir"
        return 1
    }

    # 检查端口是否已被占用
    if ! is_port_available "$port"; then
        print_warning "端口 $port 已被占用，服务可能无法启动"
    fi

    # 在后台启动服务
    local start_exit_code=0
    local log_file="$install_dir/dashboard.log"
    local pid_file="$install_dir/cloudsentinel-dashboard.pid"

    # 强制清理：先杀掉所有正在使用目标端口的进程
    if command -v fuser &>/dev/null; then
        fuser -k "${port}/tcp" &>/dev/null || true
    fi

    # 彻底清理旧进程：杀掉所有名为 dashboard 的进程，确保干净启动
    local old_pids=$(pgrep -f "dashboard" || true)
    if [ -n "$old_pids" ]; then
        print_info "正在清理冲突进程..."
        echo "$old_pids" | xargs kill -9 2>/dev/null || true
        sleep 1
    fi

    # 准备环境：确保整个目录的所有权正确
    if is_root_with_cloudsentinel; then
        chown -R cloudsentinel:root "$install_dir"
        chmod -R 770 "$install_dir"
    fi

    # 预先创建日志文件并授权
    touch "$log_file"
    is_root_with_cloudsentinel && chown cloudsentinel:root "$log_file"
    chmod 664 "$log_file"
    : > "$log_file"

    print_info "正在启动后台服务..."

    # 启动逻辑：使用 start --daemon 以守护进程模式启动
    local start_cmd="cd '$install_dir' && export CLOUDSENTINEL_PID_FILE='$pid_file' && nohup ./dashboard start --daemon"
    if is_root_with_cloudsentinel; then
        sudo -u cloudsentinel sh -c "$start_cmd > '$log_file' 2>&1 &"
    else
        (cd "$install_dir" && export CLOUDSENTINEL_PID_FILE="$pid_file" && nohup ./dashboard start --daemon > "$log_file" 2>&1 &)
    fi
    start_exit_code=$?

    # 如果 start --daemon 失败，尝试使用 -d 短选项
    sleep 2
    if grep -qE "not defined|does not exist|not found|unknown flag|invalid" "$log_file" 2>/dev/null || [ $start_exit_code -ne 0 ]; then
        print_info "检测到 --daemon 选项可能不支持，尝试使用 -d 选项..."
        pgrep -f "dashboard" | xargs kill -9 2>/dev/null || true
        sleep 1
        : > "$log_file"
        local start_cmd_short="cd '$install_dir' && export CLOUDSENTINEL_PID_FILE='$pid_file' && nohup ./dashboard start -d"
        if is_root_with_cloudsentinel; then
            sudo -u cloudsentinel sh -c "$start_cmd_short > '$log_file' 2>&1 &"
        else
            (cd "$install_dir" && export CLOUDSENTINEL_PID_FILE="$pid_file" && nohup ./dashboard start -d > "$log_file" 2>&1 &)
        fi
        start_exit_code=$?
    fi

    # 如果仍然失败，尝试使用环境变量方式直接启动
    sleep 2
    if grep -qE "not defined|does not exist|not found|unknown flag|invalid|command.*not" "$log_file" 2>/dev/null || [ $start_exit_code -ne 0 ]; then
        print_info "检测到 start 命令可能不支持，尝试使用环境变量方式启动..."
        pgrep -f "dashboard" | xargs kill -9 2>/dev/null || true
        sleep 1
        : > "$log_file"
        local start_cmd_env="cd '$install_dir' && export CLOUDSENTINEL_PID_FILE='$pid_file' && export CLOUDSENTINEL_SERVER_MODE=1 && export CLOUDSENTINEL_DAEMON_MODE=1 && nohup ./dashboard"
        if is_root_with_cloudsentinel; then
            sudo -u cloudsentinel sh -c "$start_cmd_env > '$log_file' 2>&1 &"
        else
            (cd "$install_dir" && export CLOUDSENTINEL_PID_FILE="$pid_file" && export CLOUDSENTINEL_SERVER_MODE=1 && export CLOUDSENTINEL_DAEMON_MODE=1 && nohup ./dashboard > "$log_file" 2>&1 &)
        fi
        start_exit_code=$?
    fi

    # 等待服务启动（守护进程模式需要更长时间，因为 start --daemon 会启动新进程后退出）
    sleep 4

    # 检查服务是否正在运行（使用更精确的匹配）
    local service_running=false
    local service_pid=""
    
    # 尝试多种方式检测进程
    if pgrep -f "$(basename "$binary_path")" > /dev/null 2>&1; then
        service_pid=$(pgrep -f "$(basename "$binary_path")" | head -n1)
        service_running=true
    elif pgrep -f "dashboard" > /dev/null 2>&1; then
        # 检查是否是我们的 dashboard 进程（在安装目录下运行）
        local candidate_pids=$(pgrep -f "dashboard")
        for pid in $candidate_pids; do
            if [ -n "$pid" ] && [ -d "/proc/$pid" ] 2>/dev/null; then
                local proc_cwd=$(readlink -f "/proc/$pid/cwd" 2>/dev/null || echo "")
                if [ "$proc_cwd" = "$install_dir" ] || [ -n "$(ps -p "$pid" -o cmd= | grep -F "$install_dir")" ]; then
                    service_pid="$pid"
                    service_running=true
                    break
                fi
            fi
        done
    fi

    # 再等待一下，让服务完全启动并监听端口
    sleep 2
    
    # 检查端口是否被监听
    if ! is_port_available "$port"; then
        print_success "服务已启动"
        print_info "日志文件: $install_dir/dashboard.log"
        if [ -n "$service_pid" ]; then
            print_info "进程 PID: $service_pid"
        fi
        cd "$original_dir" || true
        return 0
    elif [ "$service_running" = true ]; then
        print_warning "服务进程存在但端口未监听，请检查日志"
        print_info "日志文件: $install_dir/dashboard.log"
        if [ -f "$install_dir/dashboard.log" ]; then
            echo -e "  ${YELLOW}$(tail -n 5 "$install_dir/dashboard.log")${NC}"
        fi
        cd "$original_dir" || true
        return 1
    else
        print_error "服务启动失败，进程未运行"
        if [ -f "$install_dir/dashboard.log" ]; then
            print_info "错误日志: $install_dir/dashboard.log"
            echo -e "  ${RED}$(tail -n 10 "$install_dir/dashboard.log")${NC}"
        fi
        cd "$original_dir" || true
        return 1
    fi
}

# 主安装流程
main() {
    # 初始化全局变量
    ADMIN_USERNAME=""
    ADMIN_PASSWORD=""

    clear
    echo -e "${BOLD}${CYAN}"
    echo "CloudSentinel 安装脚本"
    echo -e "${NC}\n"

    # 获取系统信息
    get_system_info

    # 检查并安装 systemd
    set +e
    check_and_install_systemd
    set -e

    # 获取最新版本
    get_latest_version

    # 构建期望的文件名
    BINARY_NAME="dashboard-$OS_TYPE-$ARCH.tar.gz"
    SHA256_NAME="dashboard-$OS_TYPE-$ARCH.sha256"

    print_info "查找文件: $BINARY_NAME"

    # 查找二进制包
    BINARY_ASSET=$(find_asset "$BINARY_NAME")
    if [ -z "$BINARY_ASSET" ]; then
        print_error "未找到适用于 $OS_TYPE-$ARCH 的二进制包: $BINARY_NAME"
        exit 1
    fi

    BINARY_URL=$(echo "$BINARY_ASSET" | cut -d'|' -f2)

    # 查找 SHA256 文件
    SHA256_ASSET=$(find_asset "$SHA256_NAME")
    if [ -z "$SHA256_ASSET" ]; then
        print_error "未找到 SHA256 校验文件: $SHA256_NAME"
        exit 1
    fi

    SHA256_URL=$(echo "$SHA256_ASSET" | cut -d'|' -f2)

    # 创建临时目录
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT

    # 下载文件
    BINARY_FILE="$TEMP_DIR/$BINARY_NAME"
    SHA256_FILE="$TEMP_DIR/$SHA256_NAME"

    if ! download_file "$BINARY_URL" "$BINARY_FILE" "二进制包"; then
        exit 1
    fi

    if ! download_file "$SHA256_URL" "$SHA256_FILE" "SHA256 校验文件"; then
        exit 1
    fi

    # 校验文件
    if ! verify_file "$BINARY_FILE" "$SHA256_FILE"; then
        exit 1
    fi

    # 解压文件
    EXTRACT_DIR="$TEMP_DIR/extract"
    if ! extract_tar_gz "$BINARY_FILE" "$EXTRACT_DIR"; then
        exit 1
    fi

    # 查找解压后的二进制文件
    BINARY_EXE="dashboard-$OS_TYPE-$ARCH"

    EXTRACTED_BINARY="$EXTRACT_DIR/$BINARY_EXE"

    # 如果直接找不到，尝试在子目录中查找
    if [ ! -f "$EXTRACTED_BINARY" ]; then
        FOUND_BINARY=$(find "$EXTRACT_DIR" -name "$BINARY_EXE" -type f | head -n1)
        if [ -n "$FOUND_BINARY" ]; then
            EXTRACTED_BINARY="$FOUND_BINARY"
        else
            print_error "解压后未找到二进制文件: $BINARY_EXE"
            exit 1
        fi
    fi

    # 询问安装目录
    echo ""
    read -p "$(echo -e ${CYAN}请输入安装目录${NC} $(echo -e ${YELLOW}[默认: $(pwd)]${NC}): )" BASE_INSTALL_DIR
    BASE_INSTALL_DIR=${BASE_INSTALL_DIR:-$(pwd)}

    # 创建基础安装目录
    if [ ! -d "$BASE_INSTALL_DIR" ]; then
        mkdir -p "$BASE_INSTALL_DIR"
    fi

    BASE_INSTALL_DIR=$(cd "$BASE_INSTALL_DIR" && pwd)
    
    # 创建 cloudsentinel 子目录
    INSTALL_DIR="$BASE_INSTALL_DIR/cloudsentinel"
    print_info "基础安装目录: ${BOLD}$BASE_INSTALL_DIR${NC}"
    print_info "CloudSentinel 目录: ${BOLD}$INSTALL_DIR${NC}"

    # 创建 cloudsentinel 目录
    if [ ! -d "$INSTALL_DIR" ]; then
        mkdir -p "$INSTALL_DIR"
    fi

    # 创建 cloudsentinel 用户并设置权限
    if ! create_cloudsentinel_user "$INSTALL_DIR"; then
        print_warning "用户创建失败，将使用当前用户运行"
    fi

    # 确保 cloudsentinel 用户能进入安装目录（常见于安装在 /home/<user> 下）
    if ! ensure_cloudsentinel_can_access_dir "$BASE_INSTALL_DIR" "$INSTALL_DIR"; then
        print_warning "目录访问修复失败，后续初始化可能需要手动执行"
    fi

    # 复制二进制文件
    INSTALLED_BINARY="$INSTALL_DIR/dashboard"

    print_info "正在复制二进制文件..."
    if cp "$EXTRACTED_BINARY" "$INSTALLED_BINARY"; then
        chmod +x "$INSTALLED_BINARY"
        is_root_with_cloudsentinel && chown cloudsentinel:root "$INSTALLED_BINARY"
        print_success "二进制文件已复制"
    else
        print_error "复制二进制文件失败"
        exit 1
    fi

    # 查找并移动数据库文件（如果存在）
    local extracted_db=$(find "$EXTRACT_DIR" -name "database.db" -type f | head -n1)
    if [ -n "$extracted_db" ]; then
        print_info "正在预置数据库文件..."
        if cp "$extracted_db" "$INSTALL_DIR/database.db"; then
            is_root_with_cloudsentinel && chown cloudsentinel:root "$INSTALL_DIR/database.db"
            print_success "数据库文件已预置"
        else
            print_warning "移动数据库文件失败"
        fi
    fi

    # 生成随机端口
    PORT=$(generate_random_port)
    print_info "分配端口: ${BOLD}$PORT${NC}"

    # 生成 .env 文件
    generate_env_file "$INSTALL_DIR" "$PORT"
    is_root_with_cloudsentinel && chown cloudsentinel:root "$INSTALL_DIR/.env"

    # 验证 .env 文件是否创建成功
    if [ ! -f "$INSTALL_DIR/.env" ]; then
        print_error ".env 文件创建失败"
        exit 1
    fi

    # 初始化APP_KEY & JWT_SECRET
    echo ""
    if ! init_app "$INSTALLED_BINARY" "$INSTALL_DIR"; then
        print_error "应用配置初始化失败"
        echo ""
        print_info "请手动执行以下命令初始化："
        show_manual_command "key:generate"
        show_manual_command "jwt:secret"
        exit 1
    fi

    # 执行数据库迁移
    if ! run_migration "$INSTALLED_BINARY" "$INSTALL_DIR"; then
        print_error "数据库迁移失败"
        echo ""
        print_info "请手动执行以下命令迁移数据库："
        show_manual_command "migrate"
        exit 1
    fi

    # 生成管理员账号
    if ! generate_admin_account "$INSTALLED_BINARY" "$INSTALL_DIR"; then
        print_warning "生成管理员账号失败"
        echo ""
        print_info "请手动执行以下命令生成管理员账号："
        show_manual_command "generate:admin"
    fi
    
    # 设置所有文件的所有权
    if is_root_with_cloudsentinel; then
        print_info "正在设置文件所有权..."
        chown -R cloudsentinel:root "$INSTALL_DIR"
        print_success "文件所有权设置完成"
    fi

    # 启动服务
    local service_start_ok=true
    if ! start_service "$INSTALLED_BINARY" "$INSTALL_DIR" "$PORT"; then
        service_start_ok=false
        print_warning "服务启动失败，请手动启动"
        echo ""
        print_info "手动启动命令："
        echo -e "  ${CYAN}cd $INSTALL_DIR${NC}"
        if id "cloudsentinel" &>/dev/null; then
            echo -e "  ${CYAN}sudo -u cloudsentinel nohup ./dashboard start --daemon > dashboard.log 2>&1 &${NC}"
        else
            echo -e "  ${CYAN}nohup ./dashboard start --daemon > dashboard.log 2>&1 &${NC}"
        fi
    fi

    # 完成
    echo ""
    print_separator
    echo -e "${BOLD}${GREEN}✓ 安装完成！${NC}"
    print_separator
    echo ""

    # 检查服务状态
    local service_status
    if pgrep -f "$INSTALLED_BINARY" > /dev/null 2>&1 && ! is_port_available "$PORT"; then
        service_status="${GREEN}✓ 运行中${NC}"
    else
        service_status="${RED}✗ 未运行${NC}"
    fi

    # 管理员账号信息
    if [ -n "$ADMIN_USERNAME" ] && [ -n "$ADMIN_PASSWORD" ]; then
        echo -e "${BOLD}管理员账号：${NC} ${BOLD}${GREEN}$ADMIN_USERNAME${NC} / ${BOLD}${GREEN}$ADMIN_PASSWORD${NC}"
    else
        print_warning "未能自动提取管理员账号信息，请手动执行以下命令查看："
        show_manual_command "generate:admin"
    fi

    # 访问地址
    echo -e "${BOLD}访问地址：${NC}"
    
    # 优先显示公网IP
    local public_ip=$(get_public_ip)
    if [ -n "$public_ip" ] && [ "$public_ip" != "127.0.0.1" ]; then
        echo -e "  ${BOLD}${GREEN} http://$public_ip:$PORT${NC}"
    fi
    
    # 显示内网IP
    local all_ips=$(get_all_ips)
    while IFS= read -r ip; do
        if [ -n "$ip" ] && [ "$ip" != "127.0.0.1" ]; then
            # 如果是公网IP，已经显示过了，跳过
            if [ "$ip" != "$public_ip" ]; then
                echo -e "  ${BOLD} http://$ip:$PORT${NC}"
            fi
        fi
    done <<< "$all_ips"
    
    # 如果没有找到任何IP，显示本地地址
    if [ -z "$public_ip" ] && [ -z "$all_ips" ]; then
        echo -e "  ${BOLD}http://127.0.0.1:$PORT${NC}"
    fi

    # 服务状态和安装目录（一行显示）
    echo -e "${BOLD}服务状态：${NC} $service_status  |  ${BOLD}端口：${NC} $PORT  |  ${BOLD}目录：${NC} $INSTALL_DIR"
}

# 执行主函数
main "$@"

