package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	// DefaultPIDFile 默认 PID 文件路径（根据操作系统自动设置）
	DefaultPIDFile = getDefaultPIDFile()
	// MinPort 最小端口号
	MinPort = 8000
	// MaxPort 最大端口号
	MaxPort = 65535
)

// getDefaultPIDFile 根据操作系统获取默认 PID 文件路径
func getDefaultPIDFile() string {
	if os.Getenv("OS") == "Windows_NT" || filepath.Separator == '\\' {
		// Windows 系统：使用临时目录
		return filepath.Join(os.TempDir(), "cloudsentinel-dashboard.pid")
	}
	// Linux/macOS 系统：使用 /var/run
	return "/var/run/cloudsentinel-dashboard.pid"
}

// CheckPIDFile 检查 PID 文件并返回进程是否运行
func CheckPIDFile(pidFile string) (int, bool, error) {
	if pidFile == "" {
		pidFile = DefaultPIDFile
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("读取PID文件失败: %w", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, false, fmt.Errorf("解析PID失败: %w", err)
	}

	// 检查进程是否存在
	running := IsProcessRunning(pid)
	return pid, running, nil
}

// WritePID 写入 PID 文件
func WritePID(pidFile string) error {
	if pidFile == "" {
		pidFile = DefaultPIDFile
	}

	// 确保目录存在
	dir := filepath.Dir(pidFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建PID目录失败: %w", err)
	}

	pid := os.Getpid()
	data := []byte(strconv.Itoa(pid))

	if err := os.WriteFile(pidFile, data, 0644); err != nil {
		return fmt.Errorf("写入PID文件失败: %w", err)
	}

	return nil
}

// RemovePID 删除 PID 文件
func RemovePID(pidFile string) error {
	if pidFile == "" {
		pidFile = DefaultPIDFile
	}

	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除PID文件失败: %w", err)
	}

	return nil
}

// IsProcessRunning 检查进程是否正在运行
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// 发送信号 0 来检查进程是否存在
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// SendSignal 向进程发送信号
func SendSignal(pid int, sig syscall.Signal) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("查找进程失败: %w", err)
	}

	if err := process.Signal(sig); err != nil {
		return fmt.Errorf("发送信号失败: %w", err)
	}

	return nil
}

// WaitForProcessExit 等待进程退出
func WaitForProcessExit(pid int, maxWait time.Duration) bool {
	checkInterval := 200 * time.Millisecond
	elapsed := time.Duration(0)

	for elapsed < maxWait {
		if !IsProcessRunning(pid) {
			return true
		}
		time.Sleep(checkInterval)
		elapsed += checkInterval
	}

	return false
}

// StartDaemon 启动守护进程
func StartDaemon(binaryPath string, args []string, pidFile string) error {
	// 检查是否已经运行
	_, running, err := CheckPIDFile(pidFile)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("服务已在运行中")
	}

	// 构建命令
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = filepath.Dir(binaryPath)
	cmd.Env = os.Environ()

	// 启动进程
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动进程失败: %w", err)
	}

	// 写入 PID 文件
	if err := WritePID(pidFile); err != nil {
		// 如果写入失败，尝试终止进程
		_ = cmd.Process.Kill()
		return err
	}

	return nil
}

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

