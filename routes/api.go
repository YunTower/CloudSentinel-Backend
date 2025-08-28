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

	facades.Route().Post("/auth/login", authController.Login)
	facades.Route().Get("/settings/public", settingsController.GetPublicSettings)

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
	})
}
