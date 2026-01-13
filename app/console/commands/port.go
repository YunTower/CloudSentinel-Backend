package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	// MinPort 最小端口号
	MinPort = 10000
	// MaxPort 最大端口号
	MaxPort = 65535
)

// IsPortAvailable 检查端口是否可用
func IsPortAvailable(port int) bool {
	// 尝试使用 netstat 或 ss 检查端口
	// 在 Windows 上使用 netstat，在 Linux/macOS 上使用 lsof/netstat/ss
	var cmd *exec.Cmd
	if os.Getenv("OS") == "Windows_NT" || filepath.Separator == '\\' {
		// Windows 系统：使用 netstat 查找监听端口
		cmd = exec.Command("netstat", "-an")
	} else {
		// Linux/macOS 系统：使用 lsof/netstat/ss
		cmd = exec.Command("sh", "-c", fmt.Sprintf("lsof -i :%d 2>/dev/null || netstat -an 2>/dev/null | grep ':%d ' || ss -an 2>/dev/null | grep ':%d '", port, port, port))
	}

	output, err := cmd.Output()
	if err != nil {
		// 命令失败说明端口可能可用
		return true
	}

	// 检查输出中是否包含端口（Windows 格式为 :port，Linux 格式为 :port 或 :::port）
	portStr := fmt.Sprintf(":%d", port)
	outputStr := string(output)

	// Windows 上需要检查 LISTENING 状态
	if os.Getenv("OS") == "Windows_NT" || filepath.Separator == '\\' {
		// Windows: 查找 "LISTENING" 和端口号
		return !(strings.Contains(outputStr, portStr) && strings.Contains(outputStr, "LISTENING"))
	}

	// Linux/macOS: 查找端口号
	return !strings.Contains(outputStr, portStr)
}

// GenerateRandomPort 生成随机端口（8000-65535）
func GenerateRandomPort() (int, error) {
	maxAttempts := 100
	for i := 0; i < maxAttempts; i++ {
		// 使用时间戳和尝试次数生成端口
		port := MinPort + (i*17)%(MaxPort-MinPort+1)
		if IsPortAvailable(port) {
			return port, nil
		}
	}

	// 如果都不可用，返回默认端口
	return MinPort, nil
}
