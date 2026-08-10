package routes

import (
	"goravel/app/http/middleware"
	"time"

	"github.com/goravel/framework/contracts/route"
	ginfacades "github.com/goravel/gin/facades"
	"goravel/app/facades"

	"goravel/app/http/controllers"
)

type publicReadControllers struct {
	settings       *controllers.SettingsController
	servers        *controllers.ServerController
	incidents      *controllers.IncidentController
	serviceMonitor *controllers.ServiceMonitorController
}

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
	staticController := controllers.NewStaticController(controllers.AdminFiles, controllers.AdminAssetsRoot)
	certsController := controllers.NewCertsController()

	facades.Route().Prefix("api").Group(func(router route.Router) {
		registerPublicReadRoutes(router, publicReadControllers{
			settings:       settingsController,
			servers:        serverController,
			incidents:      incidentController,
			serviceMonitor: serviceMonitorController,
		})

		// Agent 引导接口虽无需登录，但不属于公开站点的只读接口集合。
		router.Middleware(middleware.Public(), middleware.PublicRateLimit(120, 1*time.Minute)).Group(func(agentBootstrapRouter route.Router) {
			// 自签 CA 公钥下载：Agent 安装时自动获取并信任（仅 PEM 文本，无敏感信息）
			agentBootstrapRouter.Get("/certs/ca", certsController.GetCA)
			// Agent 首次启动统一引导：一次请求返回 CA 等全部引导配置
			agentBootstrapRouter.Get("/agent/bootstrap", certsController.Bootstrap)
		})

		// 管理认证入口仅供管理端来源调用；后续管理 API 必须经过 AdminAuth 与 CSRF 校验。
		router.Middleware(middleware.LoginRateLimit()).Post("/auth/login", authController.Login)
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

		// 当前面板仅提供管理员登录态；认证辅助接口必须与登录 Token 使用同一 guard。
		router.Middleware(middleware.AdminAuth()).Group(func(authRouter route.Router) {
			// 认证相关
			authRouter.Prefix("/auth").Group(func(authRoute route.Router) {
				authRoute.Get("/csrf", authController.CSRFToken)
				authRoute.Middleware(middleware.VerifyCSRF()).Post("/refresh", authController.Refresh)
				authRoute.Get("/check", authController.Check)
				authRoute.Middleware(middleware.VerifyCSRF()).Post("/logout", authController.Logout)
			})
		})

		// 仅管理员接口
		router.Middleware(middleware.AdminAuth(), middleware.VerifyCSRF()).Group(func(adminRouter route.Router) {
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
				settingsRoute.Post("/alerts/templates/preview", settingsController.PreviewAlertTemplates)
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
				smRoute.Post("/ai-models", serviceMonitorController.CreateAIModels)
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
	if staticController.Available() {
		facades.Route().Fallback(staticController.ServeStatic)
	}
}

// Public 创建与管理端完全独立的公开监听路由，仅注册公开站点所需的只读接口。
func Public() route.Route {
	publicRoute := ginfacades.Route("public")
	publicRoute.SetGlobalMiddleware(append(publicRoute.GetGlobalMiddleware(), middleware.CORS()))

	publicRoute.Prefix("api").Group(func(router route.Router) {
		registerPublicReadRoutes(router, publicReadControllers{
			settings:       controllers.NewSettingsController(),
			servers:        controllers.NewServerController(),
			incidents:      controllers.NewIncidentController(),
			serviceMonitor: controllers.NewServiceMonitorController(),
		})
	})

	staticController := controllers.NewStaticController(controllers.PublicFiles, controllers.PublicAssetsRoot)
	// 即使开发构建未嵌入前端，也必须由显式 fallback 拒绝未注册的后端路径，
	// 避免 Gin 默认 NoRoute 在不同响应适配器下产生不一致状态码。
	publicRoute.Fallback(staticController.ServeStatic)

	return publicRoute
}

func registerPublicReadRoutes(router route.Router, handlers publicReadControllers) {
	// 公开接口始终以访客权限执行，绝不读取或提升管理员会话。
	router.Middleware(middleware.Public(), middleware.PublicRateLimit(120, 1*time.Minute)).Group(func(publicRouter route.Router) {
		publicRouter.Get("/settings/public", handlers.settings.GetPublicSettings)
		publicRouter.Get("/public/servers", handlers.servers.GetServers)
		publicRouter.Get("/public/incidents", handlers.incidents.GetPublic)
		publicRouter.Get("/public/service-monitors", handlers.serviceMonitor.GetPublic)
	})
}
