package routes

import (
	"reflect"
	"testing"

	contractshttp "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	"goravel/app/http/controllers"
)

type publicRouteRecorder struct {
	route.Router
	getPaths []string
}

func (r *publicRouteRecorder) Middleware(_ ...contractshttp.Middleware) route.Router {
	return r
}

func (r *publicRouteRecorder) Group(register route.GroupFunc) {
	register(r)
}

func (r *publicRouteRecorder) Get(path string, _ contractshttp.HandlerFunc) route.Action {
	r.getPaths = append(r.getPaths, path)
	return recordedAction{}
}

type recordedAction struct {
	route.Action
}

func TestRegisterPublicReadRoutes_只注册四个公开读取接口(t *testing.T) {
	recorder := &publicRouteRecorder{}
	registerPublicReadRoutes(recorder, publicReadControllers{
		settings:       &controllers.SettingsController{},
		servers:        &controllers.ServerController{},
		incidents:      &controllers.IncidentController{},
		serviceMonitor: &controllers.ServiceMonitorController{},
	})

	expected := []string{
		"/settings/public",
		"/public/servers",
		"/public/incidents",
		"/public/service-monitors",
	}
	if !reflect.DeepEqual(recorder.getPaths, expected) {
		t.Fatalf("公开监听器路由集合异常: expected=%v actual=%v", expected, recorder.getPaths)
	}
}
