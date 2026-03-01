package controllers

import (
	"goravel/app/utils"

	"github.com/goravel/framework/contracts/http"
)

// requireAdmin 控制器层管理员兜底校验，避免路由误配导致越权。
func requireAdmin(ctx http.Context) http.Response {
	guard, _ := ctx.Value("guard").(string)
	userType, _ := ctx.Value("user_type").(string)
	if guard == "admin" || userType == "admin" {
		return nil
	}

	return utils.ErrorResponse(ctx, http.StatusForbidden, "权限不足", "INSUFFICIENT_PERMISSIONS")
}
