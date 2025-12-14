package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

type RestartCommand struct {
	pidFile string
}

func NewRestartCommand() *RestartCommand {
	return &RestartCommand{
		pidFile: DefaultPIDFile,
	}
}

// Signature The name and signature of the console command.
func (c *RestartCommand) Signature() string {
	return "restart"
}

// Description The console command description.
func (c *RestartCommand) Description() string {
	return "重启 CloudSentinel Dashboard 服务"
}

// Extend The console command extend.
func (c *RestartCommand) Extend() command.Extend {
	return command.Extend{
		Flags: []command.Flag{
			&command.BoolFlag{
				Name:    "daemon",
				Aliases: []string{"d"},
				Usage:   "以守护进程模式运行",
			},
		},
	}
}

// Handle Execute the console command.
func (c *RestartCommand) Handle(ctx console.Context) error {
	PrintInfo("正在重启服务...")

	// 先停止服务
	pid, running, err := CheckPIDFile(c.pidFile)
	if err == nil && running {
		PrintInfo(fmt.Sprintf("正在停止服务 (PID: %d)...", pid))

		// 发送 SIGTERM 信号
		if err := SendSignal(pid, syscall.SIGTERM); err == nil {
			// 等待进程退出
			maxWait := 2 * time.Second
			if !WaitForProcessExit(pid, maxWait) {
				// 如果还在运行，尝试强制终止
				if IsProcessRunning(pid) {
					PrintWarning("服务未响应，尝试强制终止...")
					_ = SendSignal(pid, syscall.SIGKILL)
					time.Sleep(500 * time.Millisecond)
				}
			}
		}

		_ = RemovePID(c.pidFile)
		time.Sleep(500 * time.Millisecond)
	}

	// 再启动服务
	daemonFlag := ctx.Option("daemon") == "true" || ctx.Option("daemon") == "1"
	
	// 使用 start 命令的逻辑启动服务
	if daemonFlag {
		PrintInfo("正在以守护进程模式启动服务...")
		return c.startService(true)
	}

	PrintSuccess("服务已停止")
	PrintInfo("请使用 'start' 或 'start --daemon' 命令启动服务")

	return nil
}

// startService 启动服务的内部方法
func (c *RestartCommand) startService(daemon bool) error {
	// 获取可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		PrintError("获取可执行文件路径失败")
		return err
	}

	if daemon {
		// 守护进程模式：后台启动
		cmd := exec.Command(exePath)
		cmd.Dir = filepath.Dir(exePath)
		cmd.Env = os.Environ()

		// 启动进程
		if err := cmd.Start(); err != nil {
			PrintError(fmt.Sprintf("启动服务失败: %v", err))
			return err
		}

		// 写入 PID 文件
		if err := WritePID(c.pidFile); err != nil {
			PrintError(fmt.Sprintf("写入PID文件失败: %v", err))
			_ = cmd.Process.Kill()
			return err
		}

		// 等待一下，检查进程是否还在运行
		time.Sleep(500 * time.Millisecond)
		if IsProcessRunning(cmd.Process.Pid) {
			PrintSuccess(fmt.Sprintf("服务已启动 (PID: %d)", cmd.Process.Pid))
			return nil
		} else {
			PrintError("服务启动后立即退出，请检查日志")
			_ = RemovePID(c.pidFile)
			return fmt.Errorf("服务启动失败")
		}
	}

	return nil
}

