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

	facades.Route().Post("/auth/login", authController.Login)
	facades.Route().Get("/settings/public", settingsController.GetPublicSettings)

	facades.Route().Get("/ws/agent", wsController.HandleAgentConnection)
	facades.Route().Get("/ws/frontend", wsController.HandleFrontendConnection)

	facades.Route().Middleware(middleware.Auth()).Group(func(router route.Router) {
		router.Prefix("/settings").Get("/panel", settingsController.GetPanelSettings)
		router.Prefix("/settings").Get("/permissions", settingsController.GetPermissionsSettings)
		router.Prefix("/settings").Get("/alerts", settingsController.GetAlertsSettings)
		router.Prefix("/settings").Patch("/panel", settingsController.UpdatePanelSettings)
		router.Prefix("/settings").Patch("/permissions", settingsController.UpdatePermissionsSettings)
		router.Prefix("/settings").Patch("/alerts", settingsController.UpdateAlertsSettings)

		router.Prefix("/update").Get("/check", updateController.Check)

		router.Prefix("/auth").Get("/refresh", authController.Refresh)
		router.Prefix("/auth").Get("/check", authController.Check)

		router.Prefix("/servers").Post("", serverController.CreateServer)
		router.Prefix("/servers").Get("", serverController.GetServers)
		router.Prefix("/servers").Get("/:id", serverController.GetServerDetail)
		router.Prefix("/servers").Get("/:id/metrics/cpu", serverController.GetServerMetricsCPU)
		router.Prefix("/servers").Get("/:id/metrics/memory", serverController.GetServerMetricsMemory)
		router.Prefix("/servers").Get("/:id/metrics/disk", serverController.GetServerMetricsDisk)
		router.Prefix("/servers").Get("/:id/metrics/network", serverController.GetServerMetricsNetwork)
		router.Prefix("/servers").Patch("/:id", serverController.UpdateServer)
		router.Prefix("/servers").Delete("/:id", serverController.DeleteServer)
		router.Prefix("/servers").Post("/:id/restart", serverController.RestartServer)
	})
}
