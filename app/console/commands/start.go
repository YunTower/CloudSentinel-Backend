package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

type StartCommand struct {
	pidFile string
}

func NewStartCommand() *StartCommand {
	return &StartCommand{
		pidFile: DefaultPIDFile,
	}
}

// Signature The name and signature of the console command.
func (c *StartCommand) Signature() string {
	return "start"
}

// Description The console command description.
func (c *StartCommand) Description() string {
	return "启动 CloudSentinel Dashboard 服务"
}

// Extend The console command extend.
func (c *StartCommand) Extend() command.Extend {
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
func (c *StartCommand) Handle(ctx console.Context) error {
	// 检查是否已经运行
	pid, running, err := CheckPIDFile(c.pidFile)
	if err != nil {
		PrintError(fmt.Sprintf("检查PID文件失败: %v", err))
		return err
	}

	if running {
		PrintStatus("running", fmt.Sprintf("服务已在运行中 (PID: %d)", pid))
		return nil
	}

	// 检查 daemon 选项
	daemonOpt := ctx.Option("daemon")
	daemonShortOpt := ctx.Option("d")

	// 检查命令行参数中是否包含 --daemon 或 -d
	hasDaemonArg := false
	for _, arg := range os.Args {
		if arg == "--daemon" || arg == "-d" || strings.HasPrefix(arg, "--daemon=") || strings.HasPrefix(arg, "-d=") {
			hasDaemonArg = true
			break
		}
	}

	// 如果选项值为 "true" 或 "1"，或者命令行参数中包含 daemon 标志，则启用守护进程模式
	daemonFlag := daemonOpt == "true" || daemonOpt == "1" || daemonShortOpt == "true" || daemonShortOpt == "1" || hasDaemonArg

	if daemonFlag {
		// 守护进程模式：使用统一的启动函数
		if err := startDaemonService(c.pidFile); err != nil {
			return err
		}
	} else {
		// 前台模式
		PrintInfo("正在启动服务...")
		PrintInfo("按 Ctrl+C 停止服务")

		// 写入 PID 文件
		if err := WritePID(c.pidFile); err != nil {
			PrintError(fmt.Sprintf("写入PID文件失败: %v", err))
			return err
		}
	}

	return nil
}
