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
	bootstrap.Boot()

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
