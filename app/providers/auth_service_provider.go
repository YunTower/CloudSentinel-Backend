package providers

import (
	"github.com/goravel/framework/contracts/foundation"
)

type AuthServiceProvider struct {
}

func (receiver *AuthServiceProvider) Register(app foundation.Application) {

}

func (receiver *AuthServiceProvider) Boot(app foundation.Application) {
	// 使用默认的 ORM provider，无需自定义
}
