package routes

import (
	"goravel/app/http/middleware"
	"time"

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
	agentReportController := controllers.NewAgentReportController()
	agentTaskController := controllers.NewAgentTaskController()
	serverController := controllers.NewServerController()
	serverGroupController := controllers.NewServerGroupController()
	serverAlertController := controllers.NewServerAlertController()
	serviceMonitorController := controllers.NewServiceMonitorController()
	incidentController := controllers.NewIncidentController()
	staticController := controllers.NewStaticController()

	facades.Route().Prefix("api").Group(func(router route.Router) {
		// 公开接口
		router.Middleware(middleware.LoginRateLimit()).Post("/auth/login", authController.Login)
		router.Middleware(middleware.PublicRateLimit(120, 1*time.Minute)).Group(func(publicRouter route.Router) {
			publicRouter.Get("/settings/public", settingsController.GetPublicSettings)
			publicRouter.Get("/public/servers", serverController.GetServers)
			publicRouter.Get("/public/incidents", incidentController.GetPublic)
			publicRouter.Get("/public/service-monitors", serviceMonitorController.GetPublic)
		})
		router.Middleware(middleware.AgentRateLimit()).Group(func(agentRouter route.Router) {
			agentRouter.Post("/agent/report", agentReportController.Report)
			agentRouter.Post("/agent/tasks/pull", agentTaskController.Pull)
			agentRouter.Post("/agent/tasks/complete", agentTaskController.Complete)
		})

		// WebSocket 连接
		router.Middleware(middleware.WebSocketRateLimit()).Group(func(wsRouter route.Router) {
			wsRouter.Get("/ws/agent", wsController.HandleAgentConnection)
			wsRouter.Get("/ws/frontend", wsController.HandleFrontendConnection)
		})

		// 通用登录用户接口
		router.Middleware(middleware.Auth()).Group(func(authRouter route.Router) {
			// 认证相关
			authRouter.Prefix("/auth").Group(func(authRoute route.Router) {
				authRoute.Get("/refresh", authController.Refresh)
				authRoute.Get("/check", authController.Check)
			})
		})

		// 仅管理员接口
		router.Middleware(middleware.AdminAuth()).Group(func(adminRouter route.Router) {
			// 设置相关
			adminRouter.Prefix("/settings").Group(func(settingsRoute route.Router) {
				settingsRoute.Get("/panel", settingsController.GetPanelSettings)
				settingsRoute.Get("/permissions", settingsController.GetPermissionsSettings)
				settingsRoute.Get("/alerts", settingsController.GetAlertsSettings)
				settingsRoute.Get("/public-display", settingsController.GetPublicDisplaySettings)
				settingsRoute.Get("/public-pages", settingsController.GetPublicPagesSettings)
				settingsRoute.Patch("/panel", settingsController.UpdatePanelSettings)
				settingsRoute.Patch("/permissions", settingsController.UpdatePermissionsSettings)
				settingsRoute.Patch("/alerts", settingsController.UpdateAlertsSettings)
				settingsRoute.Patch("/public-display", settingsController.UpdatePublicDisplaySettings)
				settingsRoute.Patch("/public-pages", settingsController.UpdatePublicPagesSettings)
				settingsRoute.Post("/alerts/test", settingsController.TestAlertSettings)
			})

			// 更新相关
			adminRouter.Prefix("/update").Group(func(updateRoute route.Router) {
				updateRoute.Get("/check", updateController.Check)
				updateRoute.Get("/status", updateController.Status)
				updateRoute.Post("", updateController.UpdatePanel)
				updateRoute.Get("/agent/check", updateController.CheckAgent)
			})

			// 服务器相关
			adminRouter.Prefix("/servers").Group(func(serversRoute route.Router) {
				serversRoute.Get("", serverController.GetServers)
				serversRoute.Post("", serverController.CreateServer)
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
				serversRoute.Patch("/:id/alert-rules", serverAlertController.UpdateServerAlertRules)
				serversRoute.Post("/alert-rules/copy", serverAlertController.CopyAlertRules)
			})

			// 服务监测
			adminRouter.Prefix("/service-monitors").Group(func(smRoute route.Router) {
				smRoute.Get("", serviceMonitorController.GetAll)
				smRoute.Post("", serviceMonitorController.Create)
				smRoute.Get("/:id/results", serviceMonitorController.GetResults)
				smRoute.Patch("/:id", serviceMonitorController.Update)
				smRoute.Delete("/:id", serviceMonitorController.Delete)
			})

			// 事件时间线
			adminRouter.Prefix("/incidents").Group(func(incidentRoute route.Router) {
				incidentRoute.Get("", incidentController.GetAll)
				incidentRoute.Post("/maintenance", incidentController.CreateMaintenance)
				incidentRoute.Post("/:id/updates", incidentController.AddMaintenanceUpdate)
				incidentRoute.Post("/:id/resolve", incidentController.ResolveMaintenance)
			})

			// 分组写操作
			adminRouter.Prefix("/servers/groups").Group(func(groupsRoute route.Router) {
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
