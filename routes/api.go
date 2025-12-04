package routes

import (
	"goravel/app/http/middleware"

	"github.com/goravel/framework/contracts/route"
	"github.com/goravel/framework/facades"

	"goravel/app/http/controllers"
)

func Api() {
	authController := controllers.NewAuthController()
	settingsController := controllers.NewSettingsController()
	updateController := controllers.NewUpdateController()
	wsController := controllers.NewWebSocketController()
	serverController := controllers.NewServerController()
	serverGroupController := controllers.NewServerGroupController()
	serverAlertController := controllers.NewServerAlertController()
	staticController := controllers.NewStaticController()

	facades.Route().Prefix("api").Group(func(router route.Router) {
		router.Post("/auth/login", authController.Login)
		router.Get("/settings/public", settingsController.GetPublicSettings)

		router.Get("/ws/agent", wsController.HandleAgentConnection)
		router.Get("/ws/frontend", wsController.HandleFrontendConnection)

		router.Middleware(middleware.Auth()).Group(func(authRouter route.Router) {
			authRouter.Prefix("/settings").Get("/panel", settingsController.GetPanelSettings)
			authRouter.Prefix("/settings").Get("/permissions", settingsController.GetPermissionsSettings)
			authRouter.Prefix("/settings").Get("/alerts", settingsController.GetAlertsSettings)
			authRouter.Prefix("/settings").Patch("/panel", settingsController.UpdatePanelSettings)
			authRouter.Prefix("/settings").Patch("/permissions", settingsController.UpdatePermissionsSettings)
			authRouter.Prefix("/settings").Patch("/alerts", settingsController.UpdateAlertsSettings)
			authRouter.Prefix("/settings").Post("/alerts/test", settingsController.TestAlertSettings)

			authRouter.Prefix("/update").Get("/check", updateController.Check)
			authRouter.Prefix("/update").Get("/status", updateController.Status)
			authRouter.Prefix("/update").Post("", updateController.Update)
			authRouter.Prefix("/update").Get("/agent/check", updateController.CheckAgent)

			authRouter.Prefix("/auth").Get("/refresh", authController.Refresh)
			authRouter.Prefix("/auth").Get("/check", authController.Check)

			authRouter.Prefix("/servers").Post("", serverController.CreateServer)
			authRouter.Prefix("/servers").Get("", serverController.GetServers)
			authRouter.Prefix("/servers").Get("/:id", serverController.GetServerDetail)
			authRouter.Prefix("/servers").Get("/:id/metrics/cpu", serverController.GetServerMetricsCPU)
			authRouter.Prefix("/servers").Get("/:id/metrics/memory", serverController.GetServerMetricsMemory)
			authRouter.Prefix("/servers").Get("/:id/metrics/disk", serverController.GetServerMetricsDisk)
			authRouter.Prefix("/servers").Get("/:id/metrics/network", serverController.GetServerMetricsNetwork)
			authRouter.Prefix("/servers").Patch("/:id", serverController.UpdateServer)
			authRouter.Prefix("/servers").Delete("/:id", serverController.DeleteServer)
			authRouter.Prefix("/servers").Post("/:id/restart", serverController.RestartServer)
			authRouter.Prefix("/servers").Post("/:id/update-agent", serverController.UpdateAgent)
			authRouter.Prefix("/servers").Post("/copy-alert-rules", serverAlertController.CopyAlertRules)

			// 服务器分组管理
			authRouter.Prefix("/servers/groups").Get("", serverGroupController.GetGroups)
			authRouter.Prefix("/servers/groups").Post("", serverGroupController.CreateGroup)
			authRouter.Prefix("/servers/groups").Patch("/:id", serverGroupController.UpdateGroup)
			authRouter.Prefix("/servers/groups").Delete("/:id", serverGroupController.DeleteGroup)
		})
	})

	facades.Route().Fallback(staticController.ServeStatic)
}
