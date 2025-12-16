package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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

	// 获取可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		PrintError("获取可执行文件路径失败")
		return err
	}

	daemonFlag := ctx.Option("daemon") == "true" || ctx.Option("daemon") == "1"

	if daemonFlag {
		// 守护进程模式：后台启动
		// 设置环境变量标记，让新进程知道要启动服务器
		env := os.Environ()
		env = append(env, "CLOUDSENTINEL_SERVER_MODE=1")
		env = append(env, "CLOUDSENTINEL_DAEMON_MODE=1")

		// 重新执行程序（不带 start 参数，main 会检测环境变量启动服务）
		cmd := exec.Command(exePath)
		cmd.Dir = filepath.Dir(exePath)
		cmd.Env = env

		// 启动进程
		if err := cmd.Start(); err != nil {
			PrintError(fmt.Sprintf("启动服务失败: %v", err))
			return err
		}

		// 等待一下，检查进程是否还在运行
		time.Sleep(500 * time.Millisecond)
		if IsProcessRunning(cmd.Process.Pid) {
			PrintSuccess(fmt.Sprintf("服务已启动 (PID: %d)", cmd.Process.Pid))
			PrintInfo(fmt.Sprintf("PID文件: %s", c.pidFile))
		} else {
			PrintError("服务启动后立即退出，请检查日志")
			return fmt.Errorf("服务启动失败")
		}
	} else {
		// 前台模式：写入 PID 文件，然后返回（由 main 启动服务）
		PrintInfo("正在启动服务...")
		PrintInfo("按 Ctrl+C 停止服务")

		// 写入 PID 文件
		if err := WritePID(c.pidFile); err != nil {
			PrintError(fmt.Sprintf("写入PID文件失败: %v", err))
			return err
		}
		// 注意：不在这里 defer RemovePID，因为服务会在 main 中运行
		// PID 文件的清理应该在服务停止时进行（通过 stop 命令或信号处理）
	}

	return nil
}
