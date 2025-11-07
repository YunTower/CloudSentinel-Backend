package providers

import (
	"goravel/app/services"

	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/facades"
)

type CleanupServiceProvider struct {
	cleanupService *services.CleanupService
}

func (receiver *CleanupServiceProvider) Register(app foundation.Application) {
	receiver.cleanupService = services.NewCleanupService()
}

func (receiver *CleanupServiceProvider) Boot(app foundation.Application) {
	// 启动清理服务
	go receiver.cleanupService.Start()
	
	facades.Log().Info("数据清理服务提供者已启动")
}

