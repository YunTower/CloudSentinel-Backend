#!/bin/bash

# CloudSentinel 服务诊断脚本

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BOLD}${CYAN}CloudSentinel 服务诊断工具${NC}\n"

# 检查 .env 文件
echo -e "${BOLD}1. 检查配置文件${NC}"
if [ -f ".env" ]; then
    echo -e "${GREEN}✓ .env 文件存在${NC}"
    echo ""
    echo -e "${CYAN}关键配置：${NC}"
    APP_HOST=$(grep "^APP_HOST=" .env | cut -d'=' -f2 | tr -d '[:space:]')
    APP_PORT=$(grep "^APP_PORT=" .env | cut -d'=' -f2 | tr -d '[:space:]')
    APP_URL=$(grep "^APP_URL=" .env | cut -d'=' -f2 | tr -d '[:space:]')
    
    echo -e "  APP_HOST: ${BOLD}$APP_HOST${NC}"
    echo -e "  APP_PORT: ${BOLD}$APP_PORT${NC}"
    echo -e "  APP_URL:  ${BOLD}$APP_URL${NC}"
    echo ""
    
    if [ "$APP_HOST" != "0.0.0.0" ] && [ "$APP_HOST" != "" ]; then
        echo -e "${YELLOW}⚠ 警告: APP_HOST 不是 0.0.0.0，外网可能无法访问${NC}"
        echo -e "  当前值: $APP_HOST"
        echo -e "  建议修改为: 0.0.0.0"
        echo ""
    fi
else
    echo -e "${RED}✗ .env 文件不存在${NC}"
    exit 1
fi

# 检查服务进程
echo -e "${BOLD}2. 检查服务进程${NC}"
if pgrep -f "dashboard" > /dev/null 2>&1; then
    PID=$(pgrep -f "dashboard" | head -n1)
    echo -e "${GREEN}✓ 服务正在运行 (PID: $PID)${NC}"
    
    # 检查进程的工作目录
    PROC_CWD=$(pwdx $PID 2>/dev/null | awk '{print $2}' || readlink -f /proc/$PID/cwd 2>/dev/null || echo "未知")
    echo -e "  工作目录: $PROC_CWD"
    echo ""
else
    echo -e "${RED}✗ 服务未运行${NC}"
    echo ""
fi

# 检查端口监听
echo -e "${BOLD}3. 检查端口监听状态${NC}"
if [ -n "$APP_PORT" ]; then
    # 使用 netstat 检查
    if command -v netstat &> /dev/null; then
        LISTEN_INFO=$(netstat -tlnp 2>/dev/null | grep ":$APP_PORT " || netstat -tln 2>/dev/null | grep ":$APP_PORT ")
        if [ -n "$LISTEN_INFO" ]; then
            echo -e "${GREEN}✓ 端口 $APP_PORT 正在监听${NC}"
            echo -e "  监听信息:"
            echo "$LISTEN_INFO" | while read line; do
                echo -e "    ${CYAN}$line${NC}"
            done
            echo ""
            
            # 检查是否监听 0.0.0.0
            if echo "$LISTEN_INFO" | grep -q "0.0.0.0:$APP_PORT\|:::$APP_PORT"; then
                echo -e "${GREEN}✓ 端口绑定到 0.0.0.0，外网可以访问${NC}"
            elif echo "$LISTEN_INFO" | grep -q "127.0.0.1:$APP_PORT"; then
                echo -e "${RED}✗ 端口只绑定到 127.0.0.1，外网无法访问${NC}"
                echo -e "  解决方案: 修改 .env 文件中的 APP_HOST=0.0.0.0，然后重启服务"
            fi
            echo ""
        else
            echo -e "${RED}✗ 端口 $APP_PORT 未监听${NC}"
            echo ""
        fi
    # 使用 ss 检查
    elif command -v ss &> /dev/null; then
        LISTEN_INFO=$(ss -tlnp 2>/dev/null | grep ":$APP_PORT " || ss -tln 2>/dev/null | grep ":$APP_PORT ")
        if [ -n "$LISTEN_INFO" ]; then
            echo -e "${GREEN}✓ 端口 $APP_PORT 正在监听${NC}"
            echo -e "  监听信息:"
            echo "$LISTEN_INFO" | while read line; do
                echo -e "    ${CYAN}$line${NC}"
            done
            echo ""
            
            # 检查是否监听 0.0.0.0
            if echo "$LISTEN_INFO" | grep -q "0.0.0.0:$APP_PORT\|:::$APP_PORT"; then
                echo -e "${GREEN}✓ 端口绑定到 0.0.0.0，外网可以访问${NC}"
            elif echo "$LISTEN_INFO" | grep -q "127.0.0.1:$APP_PORT"; then
                echo -e "${RED}✗ 端口只绑定到 127.0.0.1，外网无法访问${NC}"
                echo -e "  解决方案: 修改 .env 文件中的 APP_HOST=0.0.0.0，然后重启服务"
            fi
            echo ""
        else
            echo -e "${RED}✗ 端口 $APP_PORT 未监听${NC}"
            echo ""
        fi
    # 使用 lsof 检查
    elif command -v lsof &> /dev/null; then
        LISTEN_INFO=$(lsof -i ":$APP_PORT" 2>/dev/null)
        if [ -n "$LISTEN_INFO" ]; then
            echo -e "${GREEN}✓ 端口 $APP_PORT 正在监听${NC}"
            echo -e "  监听信息:"
            echo "$LISTEN_INFO" | while read line; do
                echo -e "    ${CYAN}$line${NC}"
            done
            echo ""
        else
            echo -e "${RED}✗ 端口 $APP_PORT 未监听${NC}"
            echo ""
        fi
    else
        echo -e "${YELLOW}⚠ 无法检查端口状态（未找到 netstat/ss/lsof）${NC}"
        echo ""
    fi
fi

# 检查防火墙
echo -e "${BOLD}4. 检查防火墙状态${NC}"
if command -v ufw &> /dev/null; then
    UFW_STATUS=$(ufw status 2>/dev/null | head -n1)
    echo -e "  UFW 状态: $UFW_STATUS"
    if echo "$UFW_STATUS" | grep -q "active"; then
        if [ -n "$APP_PORT" ]; then
            if ufw status | grep -q "$APP_PORT"; then
                echo -e "${GREEN}✓ 端口 $APP_PORT 已在防火墙规则中${NC}"
            else
                echo -e "${YELLOW}⚠ 端口 $APP_PORT 可能未在防火墙规则中${NC}"
                echo -e "  建议执行: sudo ufw allow $APP_PORT/tcp"
            fi
        fi
    fi
    echo ""
elif command -v firewall-cmd &> /dev/null; then
    if firewall-cmd --state 2>/dev/null | grep -q "running"; then
        echo -e "  Firewalld 状态: 运行中"
        if [ -n "$APP_PORT" ]; then
            if firewall-cmd --list-ports 2>/dev/null | grep -q "$APP_PORT"; then
                echo -e "${GREEN}✓ 端口 $APP_PORT 已在防火墙规则中${NC}"
            else
                echo -e "${YELLOW}⚠ 端口 $APP_PORT 可能未在防火墙规则中${NC}"
                echo -e "  建议执行: sudo firewall-cmd --permanent --add-port=$APP_PORT/tcp && sudo firewall-cmd --reload"
            fi
        fi
    else
        echo -e "  Firewalld 状态: 未运行"
    fi
    echo ""
else
    echo -e "${YELLOW}⚠ 未检测到常见防火墙工具（ufw/firewalld）${NC}"
    echo -e "  请手动检查 iptables 或其他防火墙配置"
    echo ""
fi

# 检查本地连接
echo -e "${BOLD}5. 测试本地连接${NC}"
if [ -n "$APP_PORT" ]; then
    if command -v curl &> /dev/null; then
        echo -e "  测试 127.0.0.1:$APP_PORT ..."
        HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 2 "http://127.0.0.1:$APP_PORT" 2>/dev/null || echo "000")
        if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "302" ] || [ "$HTTP_CODE" = "301" ]; then
            echo -e "${GREEN}✓ 本地连接正常 (HTTP $HTTP_CODE)${NC}"
        elif [ "$HTTP_CODE" = "503" ]; then
            echo -e "${RED}✗ 本地连接返回 503 (服务不可用)${NC}"
        elif [ "$HTTP_CODE" = "000" ]; then
            echo -e "${RED}✗ 本地连接失败 (无法连接)${NC}"
        else
            echo -e "${YELLOW}⚠ 本地连接返回 HTTP $HTTP_CODE${NC}"
        fi
        echo ""
    else
        echo -e "${YELLOW}⚠ curl 未安装，跳过连接测试${NC}"
        echo ""
    fi
fi

# 检查日志
echo -e "${BOLD}6. 检查服务日志${NC}"
if [ -f "dashboard.log" ]; then
    echo -e "${GREEN}✓ 日志文件存在${NC}"
    echo -e "  最近 10 行日志:"
    echo -e "${CYAN}$(tail -n 10 dashboard.log)${NC}"
    echo ""
    
    # 检查错误
    ERROR_COUNT=$(grep -i "error\|fatal\|panic" dashboard.log 2>/dev/null | wc -l)
    if [ "$ERROR_COUNT" -gt 0 ]; then
        echo -e "${YELLOW}⚠ 发现 $ERROR_COUNT 条错误日志${NC}"
        echo -e "  最近的错误:"
        grep -i "error\|fatal\|panic" dashboard.log 2>/dev/null | tail -n 3 | while read line; do
            echo -e "    ${RED}$line${NC}"
        done
        echo ""
    fi
else
    echo -e "${YELLOW}⚠ 日志文件不存在${NC}"
    echo ""
fi

# 总结和建议
echo -e "${BOLD}${CYAN}诊断总结和建议：${NC}\n"

# 初始化建议序号计数器
SUGGESTION_NUM=1

if [ "$APP_HOST" != "0.0.0.0" ] && [ "$APP_HOST" != "" ]; then
    echo -e "${YELLOW}${SUGGESTION_NUM}. 修改 APP_HOST 配置${NC}"
    echo -e "   编辑 .env 文件，将 APP_HOST 改为 0.0.0.0"
    echo -e "   然后重启服务"
    echo ""
    SUGGESTION_NUM=$((SUGGESTION_NUM + 1))
fi

if ! pgrep -f "dashboard" > /dev/null 2>&1; then
    echo -e "${YELLOW}${SUGGESTION_NUM}. 启动服务${NC}"
    echo -e "   ./dashboard"
    echo ""
    SUGGESTION_NUM=$((SUGGESTION_NUM + 1))
fi

if [ -n "$APP_PORT" ]; then
    echo -e "${CYAN}${SUGGESTION_NUM}. 检查防火墙规则${NC}"
    echo -e "   确保端口 $APP_PORT 已开放"
    echo ""
    SUGGESTION_NUM=$((SUGGESTION_NUM + 1))
    
    echo -e "${CYAN}${SUGGESTION_NUM}. 检查云服务器安全组${NC}"
    echo -e "   如果使用云服务器，请检查安全组规则是否允许端口 $APP_PORT"
    echo ""
    SUGGESTION_NUM=$((SUGGESTION_NUM + 1))
fi

echo -e "${CYAN}${SUGGESTION_NUM}. 查看完整日志${NC}"
echo -e "   tail -f dashboard.log"
echo ""


