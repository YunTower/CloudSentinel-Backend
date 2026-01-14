package commands

import (
	"fmt"
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
	daemonOpt := ctx.Option("daemon")
	daemonFlag := daemonOpt == "true" || daemonOpt == "1"

	if daemonFlag {
		// 守护进程模式：使用统一的启动函数
		PrintInfo("正在以守护进程模式启动服务...")
		if err := startDaemonService(c.pidFile); err != nil {
			return err
		}
	} else {
		PrintSuccess("服务已停止")
		PrintInfo("请使用 'start' 或 'start --daemon' 命令启动服务")
	}

	return nil
}
