package main

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/goravel/framework/facades"

	"goravel/app/console/commands"
	"goravel/app/services"
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

func main() {
	bootstrap.Boot()

	// 检查是否应该启动服务器（守护进程模式通过环境变量标记）
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
			// start --daemon/-d: 仅启动后台进程，当前进程直接退出（避免阻塞安装脚本等场景）
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

	// 初始化服务
	_ = services.CleanupStaleLogLocks()
	services.StartPeriodicLogLockCleanup()

	// 初始化Agent数据Worker池
	_ = services.GetGlobalDataWorker()

	// 初始化日志写入队列
	_ = services.GetLogWriter()

	// 初始化性能指标批量写入缓冲区
	_ = services.GetMetricBuffer()

	// Create a channel to listen for OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start http server by facades.Route().
	go func() {
		if err := facades.Route().Run(); err != nil {
			facades.Log().Errorf("Route Run error: %v", err)
		}
	}()

	// Start schedule by facades.Schedule
	go facades.Schedule().Run()

	// Listen for the OS signal
	go func() {
		<-quit
		facades.Log().Info("接收到退出信号，开始优雅关闭...")

		// 停止性能指标批量写入缓冲区
		services.GetMetricBuffer().Stop()

		if err := facades.Route().Shutdown(); err != nil {
			facades.Log().Errorf("Route Shutdown error: %v", err)
		}
		if err := facades.Schedule().Shutdown(); err != nil {
			facades.Log().Errorf("Schedule Shutdown error: %v", err)
		}

		os.Exit(0)
	}()

	select {}
}
