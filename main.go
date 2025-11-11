package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/goravel/framework/facades"

	"goravel/app/services"
	"goravel/bootstrap"
)

func main() {
	// This bootstraps the framework and gets it ready for use.
	bootstrap.Boot()

	// 避免日志轮转失败，清理过期的日志锁文件
	if err := services.CleanupStaleLogLocks(); err != nil {
		facades.Log().Warningf("清理日志锁文件失败: %v", err)
	}

	// 启动定期清理日志锁文件的任务（每5分钟清理一次）
	services.StartPeriodicLogLockCleanup()

	// Create a channel to listen for OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start http server by facades.Route().
	go func() {
		if err := facades.Route().Run(); err != nil {
			facades.Log().Errorf("Route Run error: %v", err)
		}
	}()

	// Listen for the OS signal
	go func() {
		<-quit
		if err := facades.Route().Shutdown(); err != nil {
			facades.Log().Errorf("Route Shutdown error: %v", err)
		}

		os.Exit(0)
	}()

	select {}
}
