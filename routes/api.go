package routes

import (
	"github.com/goravel/framework/facades"

	"goravel/app/http/controllers"
)

func Api() {
	authController := controllers.NewAuthController()
	facades.Route().Post("/auth/login", authController.Login)
}
