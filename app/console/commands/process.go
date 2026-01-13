package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

var (
	// DefaultPIDFile 默认 PID 文件路径（根据操作系统自动设置）
	DefaultPIDFile = getDefaultPIDFile()
)

// getDefaultPIDFile 根据操作系统获取默认 PID 文件路径
func getDefaultPIDFile() string {
	// 允许通过环境变量覆写 PID 文件位置
	if pidFile := os.Getenv("CLOUDSENTINEL_PID_FILE"); pidFile != "" {
		return pidFile
	}

	// Windows 系统：使用临时目录
	if runtime.GOOS == "windows" || filepath.Separator == '\\' {
		return filepath.Join(os.TempDir(), "cloudsentinel-dashboard.pid")
	}

	// 非 root 用户默认使用临时目录，避免 /var/run 权限问题
	if os.Geteuid() != 0 {
		return filepath.Join(os.TempDir(), "cloudsentinel-dashboard.pid")
	}

	// root 用户使用 /var/run
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

// startDaemonService 启动守护进程服务（统一的守护进程启动逻辑）
func startDaemonService(pidFile string) error {
	// 获取可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		PrintError("获取可执行文件路径失败")
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	// 获取当前工作目录
	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = filepath.Dir(exePath)
	}

	// 设置环境变量标记
	env := os.Environ()
	env = append(env, "CLOUDSENTINEL_SERVER_MODE=1")
	env = append(env, "CLOUDSENTINEL_DAEMON_MODE=1")

	// 传递 PID 文件路径
	if pidFileEnv := os.Getenv("CLOUDSENTINEL_PID_FILE"); pidFileEnv != "" {
		env = append(env, "CLOUDSENTINEL_PID_FILE="+pidFileEnv)
	}

	// 重新执行程序，工作目录设置为当前目录
	cmd := exec.Command(exePath)
	cmd.Dir = currentDir
	cmd.Env = env

	// 启动进程
	if err := cmd.Start(); err != nil {
		PrintError(fmt.Sprintf("启动服务失败: %v", err))
		return fmt.Errorf("启动服务失败: %w", err)
	}

	// 等待一下，检查进程是否还在运行
	time.Sleep(500 * time.Millisecond)
	if IsProcessRunning(cmd.Process.Pid) {
		PrintSuccess(fmt.Sprintf("服务已启动 (PID: %d)", cmd.Process.Pid))
		PrintInfo(fmt.Sprintf("PID文件: %s", pidFile))
		return nil
	}

	PrintError("服务启动后立即退出，请检查日志")
	return fmt.Errorf("服务启动失败")
}
