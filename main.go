package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	"goravel/app/console/commands"
	"goravel/app/facades"
	"goravel/bootstrap"
)

func hasDaemonFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--daemon" || arg == "-d" {
			return true
		}
		if strings.HasPrefix(arg, "--daemon=") || strings.HasPrefix(arg, "-d=") {
			return true
		}
	}
	return false
}

func applyStartDelay() {
	msRaw := strings.TrimSpace(os.Getenv("CLOUDSENTINEL_START_DELAY_MS"))
	if msRaw == "" {
		return
	}
	_ = os.Unsetenv("CLOUDSENTINEL_START_DELAY_MS")

	ms, err := strconv.Atoi(msRaw)
	if err != nil || ms <= 0 {
		return
	}
	if ms > 60_000 {
		ms = 60_000
	}
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func main() {
	applyStartDelay()
	app := bootstrap.Boot()

	// 检查是否应该启动服务器
	shouldStartServer := os.Getenv("CLOUDSENTINEL_SERVER_MODE") == "1"

	// 检查命令行参数
	args := os.Args[1:]

	// 如果设置了守护进程模式，直接启动服务器
	if shouldStartServer {
		// 守护进程模式下需要写入 PID 文件
		if os.Getenv("CLOUDSENTINEL_DAEMON_MODE") == "1" {
			if err := commands.WritePID(commands.DefaultPIDFile); err != nil {
				facades.Log().Errorf("写入PID文件失败: %v", err)
				os.Exit(1)
			}
		}
		// 继续执行服务器启动逻辑
	} else if len(args) == 0 {
		// 如果没有参数，显示 help
		if err := facades.Artisan().Call("list"); err != nil {
			facades.Log().Errorf("执行命令失败: %v", err)
			os.Exit(1)
		}
		return
	} else {
		// 有参数，执行对应的命令
		commandArgs := strings.Join(args, " ")

		// 执行命令
		if err := facades.Artisan().Call(commandArgs); err != nil {
			facades.Log().Errorf("执行命令失败: %v", err)
			os.Exit(1)
		}

		commandName := args[0]
		if commandName == "start" {
			// start --daemon/-d: 仅启动后台进程，当前进程直接退出
			if hasDaemonFlag(args) {
				return
			}
			shouldStartServer = true
		} else {
			return
		}
	}

	if !shouldStartServer {
		return
	}

	app.Start()
}
