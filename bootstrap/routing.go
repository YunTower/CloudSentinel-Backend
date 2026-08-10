package bootstrap

import (
	"github.com/goravel/framework/contracts/route"

	"goravel/routes"
)

var publicHTTPRoute route.Route

func Routing() {
	routes.Api()
	publicHTTPRoute = routes.Public()
}
