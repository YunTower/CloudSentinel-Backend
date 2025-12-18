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

# 检查 jq
if ! command -v jq &> /dev/null; then
    print_error "jq 命令未找到，请先安装 jq"
    print_info "安装方法："
    print_info "  Ubuntu/Debian: sudo apt-get install jq"
    print_info "  CentOS/RHEL: sudo yum install jq"
    print_info "  macOS: brew install jq"
    exit 1
fi

# 检查 sha256sum
if ! command -v sha256sum &> /dev/null; then
    print_error "sha256sum 命令未找到，请先安装 sha256sum"
    print_info "安装方法："
    print_info "  Ubuntu/Debian: sudo apt-get install coreutils"
    print_info "  CentOS/RHEL: sudo yum install coreutils"
    print_info "  macOS: 通常已预装，如未安装请使用 Homebrew"
    exit 1
fi

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
    if [ -z "$LINUX_DISTRO" ] || [ -z "$PACKAGE_MANAGER" ]; then
        print_error "无法确定 Linux 发行版，无法自动安装 systemd"
        return 1
    fi

    # 检查是否有 root 权限
    if [ "$EUID" -ne 0 ]; then
        print_error "安装 systemd 需要 root 权限"
        print_info "请使用 sudo 运行此脚本，或手动安装 systemd："
        case "$PACKAGE_MANAGER" in
            apt)
                echo -e "  ${CYAN}sudo apt-get update && sudo apt-get install -y systemd${NC}"
                ;;
            pacman)
                echo -e "  ${CYAN}sudo pacman -S --noconfirm systemd${NC}"
                ;;
            dnf|yum)
                echo -e "  ${CYAN}sudo $PACKAGE_MANAGER install -y systemd${NC}"
                ;;
            zypper)
                echo -e "  ${CYAN}sudo zypper install -y systemd${NC}"
                ;;
        esac
        return 1
    fi

    print_info "正在安装 systemd..."

    case "$PACKAGE_MANAGER" in
        apt)
            if ! apt-get update; then
                print_error "更新软件包列表失败"
                return 1
            fi
            if apt-get install -y systemd systemd-sysv; then
                print_success "systemd 安装成功"
                return 0
            else
                print_error "systemd 安装失败"
                return 1
            fi
            ;;
        pacman)
            if pacman -S --noconfirm systemd; then
                print_success "systemd 安装成功"
                return 0
            else
                print_error "systemd 安装失败"
                return 1
            fi
            ;;
        dnf|yum)
            if $PACKAGE_MANAGER install -y systemd; then
                print_success "systemd 安装成功"
                return 0
            else
                print_error "systemd 安装失败"
                return 1
            fi
            ;;
        zypper)
            if zypper install -y systemd; then
                print_success "systemd 安装成功"
                return 0
            else
                print_error "systemd 安装失败"
                return 1
            fi
            ;;
        *)
            print_error "不支持的包管理器: $PACKAGE_MANAGER"
            return 1
            ;;
    esac
}

# 检查并安装 systemd（仅在 Linux 系统上）
check_and_install_systemd() {
    # 只在 Linux 系统上检查
    if [ "$OS_TYPE" != "linux" ]; then
        return 0
    fi

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
        # 安装后再次检查
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
    OS_TYPE=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    # 标准化 OS 名称
    case "$OS_TYPE" in
        linux)
            OS_TYPE="linux"
            ;;
        *)
            print_error "不支持的操作系统: $OS_TYPE（install.sh 仅支持 Linux）"
            exit 1
            ;;
    esac

    # 标准化架构名称
    case "$ARCH" in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l|arm)
            ARCH="arm"
            ;;
        i386|i686)
            ARCH="386"
            ;;
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

    # 优先使用 lsof
    if command -v lsof &> /dev/null; then
        if lsof -i ":$port" &> /dev/null; then
            return 1  # 占用
        fi
        return 0
    fi

    # 备选使用 netstat (兼容 IPv4/IPv6)
    if command -v netstat &> /dev/null; then
        if netstat -ln 2>/dev/null | grep -qE "[:.]$port "; then
            return 1  # 占用
        fi
        return 0
    fi

    # 备选使用 ss
    if command -v ss &> /dev/null; then
        if ss -ln 2>/dev/null | grep -qE "[:.]$port "; then
            return 1  # 占用
        fi
        return 0
    fi

    return 0  # 默认认为可用
}

# 获取所有可用的IP地址
get_all_ips() {
    local ips=()

    # Linux 系统
        # 使用 ip 命令（优先）
        if command -v ip &> /dev/null; then
            while IFS= read -r line; do
                ip=$(echo "$line" | grep -oE "inet ([0-9]{1,3}\.){3}[0-9]{1,3}" | awk '{print $2}' | grep -v "^127\.")
                if [ -n "$ip" ]; then
                    ips+=("$ip")
                fi
            done < <(ip addr show 2>/dev/null | grep "inet " || echo "")
        # 使用 ifconfig 命令
        elif command -v ifconfig &> /dev/null; then
            while IFS= read -r line; do
                ip=$(echo "$line" | grep -oE "inet ([0-9]{1,3}\.){3}[0-9]{1,3}" | awk '{print $2}' | grep -v "^127\.")
                if [ -n "$ip" ]; then
                    ips+=("$ip")
                fi
            done < <(ifconfig 2>/dev/null | grep "inet " || echo "")
        # 使用 hostname 命令作为备选
        elif command -v hostname &> /dev/null; then
            local hostname_ip
            hostname_ip=$(hostname -I 2>/dev/null | awk '{print $1}' | grep -v "^127\.")
            if [ -n "$hostname_ip" ]; then
                ips+=("$hostname_ip")
        fi
    fi

    # 去重并排序
    if [ ${#ips[@]} -gt 0 ]; then
        printf '%s\n' "${ips[@]}" | sort -u -t. -k1,1n -k2,2n -k3,3n -k4,4n
    else
        # 如果没有找到IP，返回localhost
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

# 以 cloudsentinel 用户执行命令
run_as_cloudsentinel() {
    local command=$1
    local working_dir=$2

    # 只在 Linux 系统上使用 sudo
    if [ "$OS_TYPE" = "linux" ] && [ "$EUID" -eq 0 ] && id "cloudsentinel" &>/dev/null; then
        # 有 root 权限且用户存在，使用 sudo -u 切换用户
        if [ -n "$working_dir" ]; then
            sudo -u cloudsentinel sh -c "cd '$working_dir' && $command"
        else
            sudo -u cloudsentinel sh -c "$command"
        fi
    else
        # 非 Linux 或非 root 或用户不存在，直接执行
        if [ -n "$working_dir" ]; then
            (cd "$working_dir" && eval "$command")
        else
            eval "$command"
        fi
    fi
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

# 初始化程序
init_app() {
    local binary_path=$1
    local install_dir=$2

    if [ -z "$binary_path" ] || [ -z "$install_dir" ]; then
        print_error "init_app 函数需要二进制文件路径和安装目录参数"
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

    print_info "正在初始化应用配置..."

    # 保存当前目录
    local original_dir=$(pwd)

    # 切换到安装目录执行命令（需要读取 .env 文件）
    cd "$install_dir" || {
        print_error "无法切换到安装目录: $install_dir"
        return 1
    }

    # 验证 .env 文件是否存在
    if [ ! -f ".env" ]; then
        print_error ".env 文件不存在: $install_dir/.env"
        cd "$original_dir" || true
        return 1
    fi

    # 临时修改 APP_ENV 为 local
    local original_app_env
    original_app_env=$(grep "^APP_ENV=" .env | cut -d'=' -f2 || echo "production")

    # 备份并修改 APP_ENV
    if grep -q "^APP_ENV=" .env; then
        sed -i.bak 's/^APP_ENV=.*/APP_ENV=local/' .env
    else
        echo "APP_ENV=local" >> .env
    fi

    # 生成 APP_KEY
    print_info "正在生成 APP_KEY..."
    local key_output
    key_output=$(run_as_cloudsentinel "\"$binary_path\" key:generate" "$install_dir" 2>&1)
    local key_exit_code=$?

    if [ $key_exit_code -eq 0 ]; then
        # 等待一下，确保文件写入完成
        sleep 0.5
        # 验证 APP_KEY 是否已写入（检查是否非空）
        local app_key_value
        app_key_value=$(grep "^APP_KEY=" .env | cut -d'=' -f2- | tr -d '[:space:]')
        if [ -n "$app_key_value" ] && [ ${#app_key_value} -ge 32 ]; then
            print_success "APP_KEY 生成成功"
        else
            print_warning "APP_KEY 可能未正确写入"
            if [ -n "$key_output" ]; then
                echo -e "  ${YELLOW}$key_output${NC}" | head -n 3
            fi
        fi
    else
        print_error "APP_KEY 生成失败"
        if [ -n "$key_output" ]; then
            echo -e "  ${RED}$key_output${NC}" | head -n 3
        fi
        # 恢复 APP_ENV
        if [ -f ".env.bak" ]; then
            mv .env.bak .env
        else
            sed -i 's/^APP_ENV=.*/APP_ENV='"$original_app_env"'/' .env
        fi
        cd "$original_dir" || true
        return 1
    fi

    # 生成 JWT_SECRET
    print_info "正在生成 JWT_SECRET..."
    local jwt_output
    jwt_output=$(run_as_cloudsentinel "\"$binary_path\" jwt:secret" "$install_dir" 2>&1)
    local jwt_exit_code=$?

    if [ $jwt_exit_code -eq 0 ]; then
        # 等待一下，确保文件写入完成
        sleep 0.5
        # 验证 JWT_SECRET 是否已写入（检查是否非空）
        local jwt_secret_value
        jwt_secret_value=$(grep "^JWT_SECRET=" .env | cut -d'=' -f2- | tr -d '[:space:]')
        if [ -n "$jwt_secret_value" ] && [ ${#jwt_secret_value} -ge 32 ]; then
            print_success "JWT_SECRET 生成成功"
        else
            print_warning "JWT_SECRET 可能未正确写入"
            if [ -n "$jwt_output" ]; then
                echo -e "  ${YELLOW}$jwt_output${NC}" | head -n 3
            fi
        fi
    else
        print_error "JWT_SECRET 生成失败"
        if [ -n "$jwt_output" ]; then
            echo -e "  ${RED}$jwt_output${NC}" | head -n 3
        fi
        # 恢复 APP_ENV
        if [ -f ".env.bak" ]; then
            mv .env.bak .env
        else
            sed -i 's/^APP_ENV=.*/APP_ENV='"$original_app_env"'/' .env
        fi
        cd "$original_dir" || true
        return 1
    fi

    # 恢复 APP_ENV 为原始值
    if [ -f ".env.bak" ]; then
        # 从备份恢复，但保留新生成的 APP_KEY 和 JWT_SECRET
        local app_key_value
        local jwt_secret_value
        app_key_value=$(grep "^APP_KEY=" .env | cut -d'=' -f2-)
        jwt_secret_value=$(grep "^JWT_SECRET=" .env | cut -d'=' -f2-)

        mv .env.bak .env
        # 更新 APP_ENV
        sed -i 's/^APP_ENV=.*/APP_ENV='"$original_app_env"'/' .env
        # 确保 APP_KEY 和 JWT_SECRET 仍然存在
        if [ -n "$app_key_value" ]; then
            if grep -q "^APP_KEY=" .env; then
                sed -i "s|^APP_KEY=.*|APP_KEY=$app_key_value|" .env
            else
                echo "APP_KEY=$app_key_value" >> .env
            fi
        fi
        if [ -n "$jwt_secret_value" ]; then
            if grep -q "^JWT_SECRET=" .env; then
                sed -i "s|^JWT_SECRET=.*|JWT_SECRET=$jwt_secret_value|" .env
            else
                echo "JWT_SECRET=$jwt_secret_value" >> .env
            fi
        fi
        rm -f .env.bak
    else
        sed -i 's/^APP_ENV=.*/APP_ENV='"$original_app_env"'/' .env
    fi

    # 恢复原目录
    cd "$original_dir" || true

    print_success "应用配置初始化完成"
}

# 执行数据库迁移
run_migration() {
    local binary_path=$1
    local install_dir=$2

    if [ -z "$binary_path" ] || [ -z "$install_dir" ]; then
        print_error "run_migration 函数需要二进制文件路径和安装目录参数"
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

    # 保存当前目录
    local original_dir=$(pwd)

    # 切换到安装目录执行命令（需要读取 .env 文件）
    cd "$install_dir" || {
        print_error "无法切换到安装目录: $install_dir"
        return 1
    }

    # 验证 .env 文件是否存在
    if [ ! -f ".env" ]; then
        print_error ".env 文件不存在: $install_dir/.env"
        cd "$original_dir" || true
        return 1
    fi

    # 临时修改 APP_ENV 为 local（如果需要）
    local original_app_env
    original_app_env=$(grep "^APP_ENV=" .env | cut -d'=' -f2 || echo "production")

    # 如果当前是 production，临时改为 local
    if [ "$original_app_env" = "production" ]; then
        if grep -q "^APP_ENV=" .env; then
            sed -i.bak 's/^APP_ENV=.*/APP_ENV=local/' .env
        else
            echo "APP_ENV=local" >> .env
        fi
    fi

    print_info "正在执行数据库迁移..."
    local migrate_output
    migrate_output=$(run_as_cloudsentinel "\"$binary_path\" migrate" "$install_dir" 2>&1)
    local migrate_exit_code=$?

    # 恢复 APP_ENV（如果修改过）
    if [ "$original_app_env" = "production" ] && [ -f ".env.bak" ]; then
        mv .env.bak .env
    fi

    if [ $migrate_exit_code -eq 0 ]; then
        print_success "数据库迁移完成"
        # 显示迁移输出（如果有重要信息）
        if [ -n "$migrate_output" ] && echo "$migrate_output" | grep -q -i "migrated\|migration"; then
            echo -e "  ${CYAN}$migrate_output${NC}" | grep -i "migrated\|migration" | head -n 5
        fi
        cd "$original_dir" || true
        return 0
    else
        print_error "数据库迁移失败"
        if [ -n "$migrate_output" ]; then
            echo -e "  ${RED}$migrate_output${NC}" | head -n 5
        fi
        cd "$original_dir" || true
        return 1
    fi
}

# 生成管理员账号
generate_admin_account() {
    local binary_path=$1
    local install_dir=$2

    if [ -z "$binary_path" ] || [ -z "$install_dir" ]; then
        print_error "generate_admin_account 函数需要二进制文件路径和安装目录参数"
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

    # 保存当前目录
    local original_dir=$(pwd)

    # 切换到安装目录执行命令（需要读取 .env 文件）
    cd "$install_dir" || {
        print_error "无法切换到安装目录: $install_dir"
        return 1
    }

    # 验证 .env 文件是否存在
    if [ ! -f ".env" ]; then
        print_error ".env 文件不存在: $install_dir/.env"
        cd "$original_dir" || true
        return 1
    fi

    # 临时修改 APP_ENV 为 local（如果需要）
    local original_app_env
    original_app_env=$(grep "^APP_ENV=" .env | cut -d'=' -f2 || echo "production")

    # 如果当前是 production，临时改为 local
    if [ "$original_app_env" = "production" ]; then
        if grep -q "^APP_ENV=" .env; then
            sed -i.bak 's/^APP_ENV=.*/APP_ENV=local/' .env
        else
            echo "APP_ENV=local" >> .env
        fi
    fi

    print_info "正在生成随机管理员账号..."
    local admin_output
    admin_output=$(run_as_cloudsentinel "\"$binary_path\" generate:admin" "$install_dir" 2>&1)
    local admin_exit_code=$?

    # 恢复 APP_ENV（如果修改过）
    if [ "$original_app_env" = "production" ] && [ -f ".env.bak" ]; then
        mv .env.bak .env
    fi

    if [ $admin_exit_code -eq 0 ]; then
        print_success "管理员账号生成完成"
        
        # 优先使用专门的机器可读行提取
        if echo "$admin_output" | grep -q "ADMIN_CREDENTIALS:"; then
            local creds=$(echo "$admin_output" | grep "ADMIN_CREDENTIALS:" | head -n1 | cut -d':' -f2-)
            ADMIN_USERNAME=$(echo "$creds" | cut -d':' -f1 | tr -d '[:space:]')
            ADMIN_PASSWORD=$(echo "$creds" | cut -d':' -f2 | tr -d '[:space:]')
        fi

        # 兜底提取逻辑
        if [ -z "$ADMIN_USERNAME" ] || [ -z "$ADMIN_PASSWORD" ]; then
            ADMIN_USERNAME=$(echo "$admin_output" | sed 's/\x1b\[[0-9;]*m//g' | grep "用户名" | sed 's/.*用户名[:： ]*//' | tr -d '[:space:]')
            ADMIN_PASSWORD=$(echo "$admin_output" | sed 's/\x1b\[[0-9;]*m//g' | grep "密码" | sed 's/.*密码[:： ]*//' | tr -d '[:space:]')
        fi
        
        cd "$original_dir" || true
        return 0
    else
        print_error "管理员账号生成失败"
        if [ -n "$admin_output" ]; then
            echo -e "  ${RED}$admin_output${NC}" | head -n 5
        fi
        cd "$original_dir" || true
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

    # 1. 强制清理：先杀掉所有正在使用目标端口的进程（确保新分配的端口可用）
    if command -v fuser &>/dev/null; then
        fuser -k "${port}/tcp" &>/dev/null || true
    fi

    # 2. 彻底清理旧进程：杀掉所有名为 dashboard 的进程，确保干净启动
    local old_pids=$(pgrep -f "dashboard" || true)
    if [ -n "$old_pids" ]; then
        print_info "正在清理冲突进程..."
        echo "$old_pids" | xargs kill -9 2>/dev/null || true
        sleep 1
    fi

    # 3. 准备环境：确保整个目录的所有权正确
    if [ "$EUID" -eq 0 ] && id "cloudsentinel" &>/dev/null; then
        chown -R cloudsentinel:root "$install_dir"
        # 允许组权限，方便 root 调试
        chmod -R 770 "$install_dir"
    fi

    # 4. 预先创建日志文件并授权（关键：修复 Permission denied）
    touch "$log_file"
    if [ "$EUID" -eq 0 ] && id "cloudsentinel" &>/dev/null; then
        chown cloudsentinel:root "$log_file"
    fi
    chmod 664 "$log_file"
    : > "$log_file"

    print_info "正在启动后台服务..."

    # 5. 启动逻辑：不再依赖二进制内部的 --daemon 参数，统一使用标准的 nohup 启动
    # 这样更稳定，且避免了 "option does not exist" 的问题
    if [ "$EUID" -eq 0 ] && id "cloudsentinel" &>/dev/null; then
        # 以 cloudsentinel 用户身份执行，但重定向操作由 root shell 完成（确保日志权限）
        # 我们先尝试执行 start 命令，如果报错则直接执行二进制
        sudo -u cloudsentinel sh -c "cd '$install_dir' && export CLOUDSENTINEL_PID_FILE='$pid_file' && nohup ./dashboard start > '$log_file' 2>&1 &"
        start_exit_code=$?
    else
        (cd "$install_dir" && export CLOUDSENTINEL_PID_FILE="$pid_file" && nohup ./dashboard start > "$log_file" 2>&1 &)
        start_exit_code=$?
    fi

    # 检查刚才的启动是否因为不支持 "start" 命令而失败
    sleep 2
    if grep -qE "not defined|does not exist|not found" "$log_file" 2>/dev/null; then
        print_info "检测到不支持 start 命令，切换到直接启动模式..."
        # 杀掉刚才启动失败可能残留在后台的任务
        pgrep -f "dashboard" | xargs kill -9 2>/dev/null || true
        : > "$log_file"
        if [ "$EUID" -eq 0 ] && id "cloudsentinel" &>/dev/null; then
            sudo -u cloudsentinel sh -c "cd '$install_dir' && export CLOUDSENTINEL_PID_FILE='$pid_file' && nohup ./dashboard > '$log_file' 2>&1 &"
        else
            (cd "$install_dir" && export CLOUDSENTINEL_PID_FILE="$pid_file" && nohup ./dashboard > "$log_file" 2>&1 &)
        fi
        start_exit_code=$?
    fi

    if [ $start_exit_code -eq 0 ]; then
        # 等待一下，让服务启动
        sleep 3

        # 检查服务是否正在运行
        local service_running=false

        # 检查进程是否存在（Linux）
        if pgrep -f "$binary_path" > /dev/null 2>&1; then
            service_running=true
        fi

                # 再等待一下，让服务完全启动
                sleep 2
        
        # 检查端口是否被监听
        if ! is_port_available "$port"; then
            print_success "服务已启动"
                    print_info "日志文件: $install_dir/dashboard.log"
            local service_pid=$(pgrep -f "$binary_path" | head -n1)
            if [ -n "$service_pid" ]; then
                print_info "进程 PID: $service_pid"
            fi
                    cd "$original_dir" || true
                    return 0
        elif [ "$service_running" = true ]; then
            print_warning "服务进程存在但端口未监听，请检查日志"
            print_info "日志文件: $install_dir/dashboard.log"
            if [ -f "$install_dir/dashboard.log" ]; then
                echo -e "  ${YELLOW}$(tail -n 3 "$install_dir/dashboard.log")${NC}"
            fi
            cd "$original_dir" || true
            return 1
        else
            print_error "服务启动失败，进程未运行"
            if [ -f "$install_dir/dashboard.log" ]; then
                print_info "错误日志: $install_dir/dashboard.log"
                echo -e "  ${RED}$(tail -n 5 "$install_dir/dashboard.log")${NC}"
            fi
            cd "$original_dir" || true
            return 1
        fi
    else
        print_error "服务启动失败"
        if [ -f "$install_dir/dashboard.log" ]; then
            print_info "错误日志: $install_dir/dashboard.log"
            echo -e "  ${RED}$(tail -n 5 "$install_dir/dashboard.log")${NC}"
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
    if [ "$OS_TYPE" = "linux" ]; then
        # 临时禁用错误退出，允许 systemd 检查失败时继续执行
        set +e
        check_and_install_systemd
        set -e
    fi

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

    # 创建 cloudsentinel 用户并设置权限（Linux 系统）
    if [ "$OS_TYPE" = "linux" ]; then
        if ! create_cloudsentinel_user "$INSTALL_DIR"; then
            print_warning "用户创建失败，将使用当前用户运行"
        fi
    fi

    # 确保 cloudsentinel 用户能进入安装目录（常见于安装在 /home/<user> 下）
    if [ "$OS_TYPE" = "linux" ]; then
        if ! ensure_cloudsentinel_can_access_dir "$BASE_INSTALL_DIR" "$INSTALL_DIR"; then
            print_warning "目录访问修复失败，后续初始化可能需要手动执行"
        fi
    fi

    # 复制二进制文件
    INSTALLED_BINARY="$INSTALL_DIR/dashboard"

    print_info "正在复制二进制文件..."
    if cp "$EXTRACTED_BINARY" "$INSTALLED_BINARY"; then
        # 设置可执行权限
            chmod +x "$INSTALLED_BINARY"
        # 如果是 Linux 且有 cloudsentinel 用户，设置所有权
        if [ "$EUID" -eq 0 ] && id "cloudsentinel" &>/dev/null; then
            chown cloudsentinel:root "$INSTALLED_BINARY"
        fi
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
            if [ "$OS_TYPE" = "linux" ] && [ "$EUID" -eq 0 ] && id "cloudsentinel" &>/dev/null; then
                chown cloudsentinel:root "$INSTALL_DIR/database.db"
            fi
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
    
    # 如果是 Linux 且有 cloudsentinel 用户，设置 .env 文件所有权
    if [ "$OS_TYPE" = "linux" ] && [ "$EUID" -eq 0 ] && id "cloudsentinel" &>/dev/null; then
        chown cloudsentinel:root "$INSTALL_DIR/.env"
    fi

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
        if [ "$OS_TYPE" = "linux" ] && id "cloudsentinel" &>/dev/null; then
            echo -e "  ${CYAN}sudo -u cloudsentinel $INSTALL_DIR/dashboard key:generate${NC}"
            echo -e "  ${CYAN}sudo -u cloudsentinel $INSTALL_DIR/dashboard jwt:secret${NC}"
        else
        echo -e "  ${CYAN}cd $INSTALL_DIR${NC}"
        echo -e "  ${CYAN}./dashboard key:generate${NC}"
        echo -e "  ${CYAN}./dashboard jwt:secret${NC}"
        fi
        exit 1
    fi

    # 执行数据库迁移
    if ! run_migration "$INSTALLED_BINARY" "$INSTALL_DIR"; then
        print_error "数据库迁移失败"
        echo ""
        print_info "请手动执行以下命令迁移数据库："
        if [ "$OS_TYPE" = "linux" ] && id "cloudsentinel" &>/dev/null; then
            echo -e "  ${CYAN}sudo -u cloudsentinel $INSTALL_DIR/dashboard migrate${NC}"
        else
        echo -e "  ${CYAN}cd $INSTALL_DIR${NC}"
        echo -e "  ${CYAN}./dashboard migrate${NC}"
        fi
        exit 1
    fi

    # 生成管理员账号
    if ! generate_admin_account "$INSTALLED_BINARY" "$INSTALL_DIR"; then
        print_warning "生成管理员账号失败"
        echo ""
        print_info "请手动执行以下命令生成管理员账号："
        if [ "$OS_TYPE" = "linux" ] && id "cloudsentinel" &>/dev/null; then
            echo -e "  ${CYAN}sudo -u cloudsentinel $INSTALL_DIR/dashboard generate:admin${NC}"
        else
        echo -e "  ${CYAN}cd $INSTALL_DIR${NC}"
        echo -e "  ${CYAN}./dashboard generate:admin${NC}"
        fi
    fi
    
    # 设置所有文件的所有权（如果是 Linux 且有 cloudsentinel 用户）
    if [ "$OS_TYPE" = "linux" ] && [ "$EUID" -eq 0 ] && id "cloudsentinel" &>/dev/null; then
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
        echo -e "  ${CYAN}nohup ./dashboard start > dashboard.log 2>&1 &${NC}"
    fi

    # 完成
    echo ""
    print_separator
    echo -e "${BOLD}${GREEN}✓ 安装完成！${NC}"
    print_separator
    echo ""

    # 验证 .env 文件关键配置
    local app_key_check
    local jwt_secret_check
    if [ -f "$INSTALL_DIR/.env" ]; then
        app_key_check=$(grep "^APP_KEY=" "$INSTALL_DIR/.env" | cut -d'=' -f2- | tr -d '[:space:]')
        jwt_secret_check=$(grep "^JWT_SECRET=" "$INSTALL_DIR/.env" | cut -d'=' -f2- | tr -d '[:space:]')
    fi

    echo -e "${BOLD}安装与服务信息：${NC}"
    echo -e "  ${CYAN}安装目录:${NC}     ${BOLD}$INSTALL_DIR${NC}"
    echo -e "  ${CYAN}服务端口:${NC}     ${BOLD}$PORT${NC}"
    if [ "$OS_TYPE" = "linux" ] && id "cloudsentinel" &>/dev/null; then
        echo -e "  ${CYAN}运行用户:${NC}     ${BOLD}cloudsentinel${NC}"
    fi
    echo -e "  ${CYAN}日志文件:${NC}     $INSTALL_DIR/dashboard.log"
    echo ""

    # 管理员账号信息（置于核心位置）
    if [ -n "$ADMIN_USERNAME" ] && [ -n "$ADMIN_PASSWORD" ]; then
        echo -e "  ${GREEN}${BOLD}管理员账号：${NC}"
        echo -e "    ${CYAN}用户名:${NC}   ${BOLD}${GREEN}$ADMIN_USERNAME${NC}"
        echo -e "    ${CYAN}密码:${NC}     ${BOLD}${GREEN}$ADMIN_PASSWORD${NC}"
        echo ""
    else
        echo -e "  ${YELLOW}未能解析管理员账号（已写入数据库），可手动重置：${NC}"
        echo -e "    sudo -u cloudsentinel $INSTALL_DIR/dashboard generate:admin"
        echo ""
    fi

    if [ -n "$app_key_check" ] && [ ${#app_key_check} -ge 32 ] && [ -n "$jwt_secret_check" ] && [ ${#jwt_secret_check} -ge 32 ]; then
        echo -e "  ${GREEN}✓ 系统密钥已配置${NC}"
    fi

    echo -e "  ${CYAN}服务状态：${NC}"
    if pgrep -f "$INSTALLED_BINARY" > /dev/null 2>&1 && ! is_port_available "$PORT"; then
        echo -e "    ${GREEN}✓ 正在运行${NC}"
    else
        echo -e "    ${RED}✗ 未运行${NC}"
    fi
    
    echo -e "  ${CYAN}访问地址：${NC}"
    local all_ips=$(get_all_ips)
    while IFS= read -r ip; do
        [ -n "$ip" ] && echo -e "    ${BOLD}http://$ip:$PORT${NC}"
    done <<< "$all_ips"

    echo ""
    echo -e "  ${CYAN}账号重置:${NC}   sudo -u cloudsentinel $INSTALL_DIR/dashboard panel:info"
    echo ""
    echo -e "${BOLD}管理命令：${NC}"
    if [ "$EUID" -eq 0 ] && id "cloudsentinel" &>/dev/null; then
        echo -e "  ${CYAN}停止:${NC} sudo -u cloudsentinel $INSTALL_DIR/dashboard stop"
        echo -e "  ${CYAN}启动:${NC} sudo -u cloudsentinel sh -c \"cd '$INSTALL_DIR' && CLOUDSENTINEL_PID_FILE='$INSTALL_DIR/cloudsentinel-dashboard.pid' nohup ./dashboard start > dashboard.log 2>&1 &\""
        echo -e "  ${CYAN}日志:${NC} tail -f $INSTALL_DIR/dashboard.log"
    else
        echo -e "  ${CYAN}停止:${NC} $INSTALL_DIR/dashboard stop"
        echo -e "  ${CYAN}启动:${NC} cd '$INSTALL_DIR' && CLOUDSENTINEL_PID_FILE='$INSTALL_DIR/cloudsentinel-dashboard.pid' nohup ./dashboard start > dashboard.log 2>&1 &"
        echo -e "  ${CYAN}日志:${NC} tail -f $INSTALL_DIR/dashboard.log"
    fi
    echo ""
    print_separator
    echo ""
}

# 执行主函数
main "$@"

