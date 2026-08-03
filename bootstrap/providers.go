package bootstrap

import (
	"github.com/goravel/framework/auth"
	"github.com/goravel/framework/cache"
	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/database"
	"github.com/goravel/framework/filesystem"
	"github.com/goravel/framework/hash"
	"github.com/goravel/framework/http"
	"github.com/goravel/framework/log"
	"github.com/goravel/framework/queue"
	"github.com/goravel/framework/route"
	"github.com/goravel/framework/schedule"
	"github.com/goravel/framework/validation"
	"github.com/goravel/framework/view"
	"github.com/goravel/gin"
	"github.com/goravel/sqlite"
)

// Providers 返回应用启用的功能模块。Config、Artisan 与 Process 由 v1.17 基础内核提供。
func Providers() []foundation.ServiceProvider {
	return []foundation.ServiceProvider{
		&log.ServiceProvider{},
		&view.ServiceProvider{},
		&sqlite.ServiceProvider{},
		&database.ServiceProvider{},
		&cache.ServiceProvider{},
		&filesystem.ServiceProvider{},
		&http.ServiceProvider{},
		&route.ServiceProvider{},
		&schedule.ServiceProvider{},
		&queue.ServiceProvider{},
		&auth.ServiceProvider{},
		&hash.ServiceProvider{},
		&validation.ServiceProvider{},
		&gin.ServiceProvider{},
	}
}
