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

func main() {
	bootstrap.Boot()

	// 检查是否应该启动服务器（守护进程模式通过环境变量标记）
	shouldStartServer := os.Getenv("CLOUDSENTINEL_SERVER_MODE") == "1"

	// 检查命令行参数
	args := os.Args[1:]

	// 如果设置了服务器模式环境变量（守护进程模式），直接启动服务器
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
		// 构建命令字符串（包含所有参数和选项）
		commandArgs := strings.Join(args, " ")

		// 执行命令
		if err := facades.Artisan().Call(commandArgs); err != nil {
			facades.Log().Errorf("执行命令失败: %v", err)
			os.Exit(1)
		}

		// 检查执行的命令是否是 "start"，如果是则启动服务器
		// 注意：这里需要检查原始参数，因为 commandArgs 可能包含选项
		commandName := args[0]
		if commandName == "start" {
			shouldStartServer = true
			// 前台模式下，start 命令已经写入 PID 文件，继续启动服务器
		} else {
			// 其他命令执行后直接退出
			return
		}
	}

	// 只有 shouldStartServer 为 true 时才启动服务器
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
