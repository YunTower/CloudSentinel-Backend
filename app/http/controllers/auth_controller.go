package controllers

import (
	"goravel/app/http/requests/auth"
	"goravel/app/models"
	"goravel/app/repositories"
	"goravel/app/services"
	"goravel/app/utils"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

type UserInfo struct {
	ID              string
	Type            string
	Guard           string
	IsAuthenticated bool
}

type AuthController struct {
	Type     string `form:"type" json:"type"`
	Password string `form:"password" json:"password"`
	Username string `form:"username" json:"username"`
	Remember bool   `form:"remember" json:"remember"`
}

func NewAuthController() *AuthController {
	return &AuthController{}
}

// getUserInfo 从上下文获取用户信息，简化处理
func getUserInfo(ctx http.Context) *UserInfo {
	userID, _ := ctx.Value("user_id").(string)
	userType, _ := ctx.Value("user_type").(string)
	guard, _ := ctx.Value("guard").(string)
	isAuthenticated, _ := ctx.Value("is_authenticated").(bool)

	return &UserInfo{
		ID:              userID,
		Type:            userType,
		Guard:           guard,
		IsAuthenticated: isAuthenticated,
	}
}

// requireAuth 要求用户必须认证，统一错误处理
func requireAuth(ctx http.Context) (*UserInfo, http.Response) {
	// 检查是否已认证
	isAuthenticated, ok := ctx.Value("is_authenticated").(bool)

	if !ok || !isAuthenticated {
		return nil, utils.ErrorResponse(ctx, 401, "用户未认证", "UNAUTHENTICATED")
	}

	userInfo := getUserInfo(ctx)
	return userInfo, nil
}

func (r *AuthController) Login(ctx http.Context) http.Response {
	var loginPost auth.LoginPostRequest

	// 使用 ValidateRequest 方法验证表单请求
	errors, err := ctx.Request().ValidateRequest(&loginPost)
	if err != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "验证器创建失败",
			"error":   err.Error(),
		})
	}

	if errors != nil {
		return ctx.Response().Status(422).Json(http.Json{
			"status":  false,
			"message": "验证失败",
			"errors":  errors,
		})
	}

	// 获取客户端IP
	ip := ctx.Request().Ip()
	lockoutService := services.NewLoginLockoutService()
	settingRepo := repositories.GetSystemSettingRepository()

	// 检查IP是否被锁定
	isLocked, err := lockoutService.IsIPLocked(ip)
	if err != nil {
		// 如果检查锁定失败，记录错误但不阻止登录
		facades.Log().Errorf("检查IP锁定状态失败: %v", err)
	} else if isLocked {
		return ctx.Response().Status(429).Json(http.Json{
			"status":  false,
			"message": "IP已被锁定，请稍后再试",
			"code":    "IP_LOCKED",
		})
	}

	// 查询用户名
	userName := settingRepo.GetValue("admin_username", "admin")
	if userName == "" {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "用户名配置不存在",
			"code":    "CONFIG_ERROR",
			"error":   "Admin username not configured",
		})
	}

	// 查询密码哈希
	userPasswordHash := settingRepo.GetValue("admin_password_hash", "")
	if userPasswordHash == "" {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "密码配置不存在",
		})
	}

	// 验证用户名
	if loginPost.Username != userName {
		if err := lockoutService.IncrementFailedAttempts(ip); err != nil {
			facades.Log().Errorf("增加登录失败计数失败: %v", err)
		}
		return ctx.Response().Status(401).Json(http.Json{
			"status":  false,
			"message": "用户名错误",
		})
	}

	// 验证密码哈希
	if facades.Hash().Check(loginPost.Password, userPasswordHash) != true {
		if err := lockoutService.IncrementFailedAttempts(ip); err != nil {
			facades.Log().Errorf("增加登录失败计数失败: %v", err)
		}
		return ctx.Response().Status(401).Json(http.Json{
			"status":  false,
			"message": "密码错误",
		})
	}

	// 创建用户模型用于认证
	ua := ctx.Request().Header("User-Agent")

	// 创建 User 模型实例
	user := &models.User{
		Username: loginPost.Username,
		Type:     "admin",
		IP:       ip,
		UA:       ua,
	}
	user.ID = 1 // 管理员使用固定 ID 1

	token, tokenErr := facades.Auth(ctx).Guard("admin").Login(user)
	if tokenErr != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "Token生成失败",
			"error":   tokenErr.Error(),
		})
	}

	// 登录成功，清除失败计数
	if err := lockoutService.ClearFailedAttempts(ip); err != nil {
		facades.Log().Errorf("清除登录失败计数失败: %v", err)
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "登录成功",
		"data": map[string]any{
			"token":    token,
			"username": loginPost.Username,
			"type":     "admin",
		},
	})
}

func (r *AuthController) Refresh(ctx http.Context) http.Response {
	// 从请求头获取 Authorization token
	authHeader := ctx.Request().Header("Authorization")
	if authHeader == "" {
		return ctx.Response().Status(401).Json(http.Json{
			"status":  false,
			"message": "缺少认证令牌",
		})
	}

	// 移除 "Bearer " 前缀（如果存在）
	token := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	// 解析 Token
	payload, err := facades.Auth(ctx).Parse(token)
	if err != nil {
		// 检查是否是 Token 过期错误
		if err.Error() == "token has expired" || err.Error() == "token expired" {
			return ctx.Response().Status(401).Json(http.Json{
				"status":  false,
				"message": "Token已过期",
				"code":    "TOKEN_EXPIRED",
				"error":   "Token has expired, please login again",
			})
		}

		return ctx.Response().Status(401).Json(http.Json{
			"status":  false,
			"message": "Token无效",
			"code":    "TOKEN_INVALID",
			"error":   "Invalid token format or signature",
		})
	}

	// 刷新 Token
	newToken, refreshErr := facades.Auth(ctx).Refresh()
	if refreshErr != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "Token刷新失败",
			"error":   refreshErr.Error(),
		})
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "Token刷新成功",
		"data": map[string]any{
			"token":      newToken,
			"user_id":    payload.Key,
			"guard":      payload.Guard,
			"expires_at": payload.ExpireAt.Unix(),
		},
	})
}

func (r *AuthController) Check(ctx http.Context) http.Response {
	// 使用辅助函数获取用户信息，减少重复代码
	userInfo, errResponse := requireAuth(ctx)
	if errResponse != nil {
		return errResponse
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "用户已认证",
		"data": map[string]any{
			"user_id":   userInfo.ID,
			"guard":     userInfo.Guard,
			"user_type": userInfo.Type,
			"is_valid":  true,
		},
	})
}
