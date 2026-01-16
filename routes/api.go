package routes

import (
	"os"

	"goravel/app/http/middleware"

	"github.com/goravel/framework/contracts/route"
	"github.com/goravel/framework/facades"

	"goravel/app/http/controllers"
)

func Api() {
	// 初始化控制器
	authController := controllers.NewAuthController()
	settingsController := controllers.NewSettingsController()
	updateController := controllers.NewUpdateController()
	wsController := controllers.NewWebSocketController()
	serverController := controllers.NewServerController()
	serverGroupController := controllers.NewServerGroupController()
	serverAlertController := controllers.NewServerAlertController()
	staticController := controllers.NewStaticController()

	facades.Route().Prefix("api").Group(func(router route.Router) {
		// 公开接口
		router.Post("/auth/login", authController.Login)
		router.Get("/settings/public", settingsController.GetPublicSettings)

		// WebSocket 连接
		router.Get("/ws/agent", wsController.HandleAgentConnection)
		router.Get("/ws/frontend", wsController.HandleFrontendConnection)

		router.Middleware(middleware.Auth()).Group(func(authRouter route.Router) {
			// 认证相关
			authRouter.Prefix("/auth").Group(func(authRoute route.Router) {
				authRoute.Get("/refresh", authController.Refresh)
				authRoute.Get("/check", authController.Check)
			})

			// 设置相关
			authRouter.Prefix("/settings").Group(func(settingsRoute route.Router) {
				settingsRoute.Get("/panel", settingsController.GetPanelSettings)
				settingsRoute.Get("/permissions", settingsController.GetPermissionsSettings)
				settingsRoute.Get("/alerts", settingsController.GetAlertsSettings)
				settingsRoute.Patch("/panel", settingsController.UpdatePanelSettings)
				settingsRoute.Patch("/permissions", settingsController.UpdatePermissionsSettings)
				settingsRoute.Patch("/alerts", settingsController.UpdateAlertsSettings)
				settingsRoute.Post("/alerts/test", settingsController.TestAlertSettings)
			})

			// 更新相关
			authRouter.Prefix("/update").Group(func(updateRoute route.Router) {
				updateRoute.Get("/check", updateController.Check)
				updateRoute.Get("/status", updateController.Status)
				updateRoute.Post("", updateController.UpdatePanel)
				updateRoute.Get("/agent/check", updateController.CheckAgent)
			})

			// 服务器相关
			authRouter.Prefix("/servers").Group(func(serversRoute route.Router) {
				// 服务器基础操作
				serversRoute.Post("", serverController.CreateServer)
				serversRoute.Get("", serverController.GetServers)
				serversRoute.Get("/:id", serverController.GetServerDetail)
				serversRoute.Patch("/:id", serverController.UpdateServer)
				serversRoute.Delete("/:id", serverController.DeleteServer)

				// 服务器指标
				serversRoute.Get("/:id/metrics/cpu", serverController.GetServerMetricsCPU)
				serversRoute.Get("/:id/metrics/memory", serverController.GetServerMetricsMemory)
				serversRoute.Get("/:id/metrics/disk", serverController.GetServerMetricsDisk)
				serversRoute.Get("/:id/metrics/network", serverController.GetServerMetricsNetwork)

				// 服务器操作
				serversRoute.Post("/:id/agent/restart", serverController.RestartAgent)
				serversRoute.Post("/:id/agent/update", updateController.UpdateAgent)
				serversRoute.Post("/:id/agent/reset-key", serverController.ResetAgentKey)

				// 服务器告警规则
				serversRoute.Get("/:id/alert-rules", serverAlertController.GetServerAlertRules)
				serversRoute.Post("/alert-rules/copy", serverAlertController.CopyAlertRules)
			})

			// 服务器分组管理
			authRouter.Prefix("/servers/groups").Group(func(groupsRoute route.Router) {
				groupsRoute.Get("", serverGroupController.GetGroups)
				groupsRoute.Post("", serverGroupController.CreateGroup)
				groupsRoute.Patch("/:id", serverGroupController.UpdateGroup)
				groupsRoute.Delete("/:id", serverGroupController.DeleteGroup)
			})
		})
	})

	// 静态文件服务
	if hasEmbeddedFiles() {
		facades.Route().Fallback(staticController.ServeStatic)
	}
}

// hasEmbeddedFiles 检查是否嵌入了前端文件
func hasEmbeddedFiles() bool {
	// 检查 PublicFiles 是否已初始化且有内容
	entries, err := controllers.PublicFiles.ReadDir("public")
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// isProductionMode 检查是否在生产模式
func isProductionMode() bool {
	env := os.Getenv("APP_ENV")
	return env == "production"
}
