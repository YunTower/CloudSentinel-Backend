package commands

import (
	"fmt"
	"syscall"
	"time"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

type StopCommand struct {
	pidFile string
}

func NewStopCommand() *StopCommand {
	return &StopCommand{
		pidFile: DefaultPIDFile,
	}
}

// Signature The name and signature of the console command.
func (c *StopCommand) Signature() string {
	return "stop"
}

// Description The console command description.
func (c *StopCommand) Description() string {
	return "停止 CloudSentinel Dashboard 服务"
}

// Extend The console command extend.
func (c *StopCommand) Extend() command.Extend {
	return command.Extend{}
}

// Handle Execute the console command.
func (c *StopCommand) Handle(ctx console.Context) error {
	// 读取 PID
	pid, running, err := CheckPIDFile(c.pidFile)
	if err != nil {
		PrintError(fmt.Sprintf("检查PID文件失败: %v", err))
		return err
	}

	if !running {
		PrintStatus("stopped", "服务未运行")
		// 清理可能残留的 PID 文件
		_ = RemovePID(c.pidFile)
		return nil
	}

	PrintInfo(fmt.Sprintf("正在停止服务 (PID: %d)...", pid))

	// 发送 SIGTERM 信号
	if err := SendSignal(pid, syscall.SIGTERM); err != nil {
		PrintError(fmt.Sprintf("发送停止信号失败: %v", err))
		return err
	}

	// 等待进程退出
	maxWait := 3 * time.Second
	if WaitForProcessExit(pid, maxWait) {
		PrintSuccess("服务已停止")
		_ = RemovePID(c.pidFile)
		return nil
	}

	// 如果还在运行，尝试强制终止
	if IsProcessRunning(pid) {
		PrintWarning("服务未响应，尝试强制终止...")
		if err := SendSignal(pid, syscall.SIGKILL); err != nil {
			PrintError(fmt.Sprintf("强制终止失败: %v", err))
			return err
		}

		// 再等待一下
		time.Sleep(500 * time.Millisecond)
		if !IsProcessRunning(pid) {
			PrintWarning("服务已强制终止")
			_ = RemovePID(c.pidFile)
			return nil
		}

		PrintError("无法停止服务")
		return fmt.Errorf("无法停止服务")
	}

	return nil
}

