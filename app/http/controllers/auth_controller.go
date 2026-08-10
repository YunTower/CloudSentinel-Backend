package controllers

import (
	"goravel/app/http/middleware"
	"goravel/app/http/requests/auth"
	"goravel/app/models"
	"goravel/app/repositories"
	"goravel/app/services"
	"goravel/app/utils"

	"github.com/goravel/framework/contracts/http"
	"goravel/app/facades"
)

type UserInfo struct {
	ID              string
	Type            string
	Guard           string
	IsAuthenticated bool
}

type authCookieConfig struct {
	Path     string
	Domain   string
	Secure   bool
	SameSite string
}

// currentAuthCookieConfig 读取认证 Cookie 使用的基础配置。
func currentAuthCookieConfig() authCookieConfig {
	config := facades.Config()

	path := config.GetString("session.path")
	if path == "" {
		path = "/"
	}

	return authCookieConfig{
		Path:     path,
		Domain:   config.GetString("session.domain"),
		Secure:   config.GetBool("session.secure"),
		SameSite: normalizeCookieSameSite(config.GetString("session.same_site")),
	}
}

// normalizeCookieSameSite 规范化 SameSite 配置，非法值回退到 lax。
func normalizeCookieSameSite(value string) string {
	switch value {
	case "strict", "Strict", "STRICT":
		return "strict"
	case "none", "None", "NONE":
		return "none"
	case "lax", "Lax", "LAX":
		return "lax"
	default:
		return "lax"
	}
}

// buildAuthCookie 根据统一配置构造认证相关 Cookie。
func buildAuthCookie(name, value string, maxAge int, httpOnly bool, config authCookieConfig) http.Cookie {
	return http.Cookie{
		Name:     name,
		Value:    value,
		Path:     config.Path,
		Domain:   config.Domain,
		MaxAge:   maxAge,
		Secure:   config.Secure,
		HttpOnly: httpOnly,
		SameSite: config.SameSite,
	}
}

func setAuthenticationCookies(ctx http.Context, token string, remember bool) error {
	csrfToken, err := middleware.RequireCSRFToken()
	if err != nil {
		return err
	}
	maxAge := 0
	if remember {
		maxAge = 14 * 24 * 60 * 60
	}
	cookieConfig := currentAuthCookieConfig()
	ctx.Response().Cookie(buildAuthCookie(middleware.AuthTokenCookieName, token, maxAge, true, cookieConfig))
	ctx.Response().Cookie(buildAuthCookie(middleware.CSRFTokenCookieName, csrfToken, maxAge, false, cookieConfig))
	return nil
}

func clearAuthenticationCookies(ctx http.Context) {
	cookieConfig := currentAuthCookieConfig()
	ctx.Response().Cookie(buildAuthCookie(middleware.AuthTokenCookieName, "", -1, true, cookieConfig))
	ctx.Response().Cookie(buildAuthCookie(middleware.CSRFTokenCookieName, "", -1, false, cookieConfig))
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

	// 认证配置动态化：登录前同步 system_settings 中的 JWT 密钥/有效期（DB 优先）
	services.SyncAuthSettingsFromDB()

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
	if err := setAuthenticationCookies(ctx, token, loginPost.Remember); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "设置认证 Cookie 失败", err)
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "登录成功",
		"data": map[string]any{
			"username": loginPost.Username,
			"type":     "admin",
		},
	})
}

func (r *AuthController) Refresh(ctx http.Context) http.Response {
	// 认证配置动态化：刷新前同步（管理员轮换密钥后旧 token 立即失效，需重新登录）
	services.SyncAuthSettingsFromDB()

	token := ctx.Request().Cookie(middleware.AuthTokenCookieName)
	if token == "" {
		return ctx.Response().Status(401).Json(http.Json{
			"status":  false,
			"message": "缺少认证令牌",
		})
	}

	// 解析 Token
	payload, err := facades.Auth(ctx).Guard(middleware.AdminGuardName).Parse(token)
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
	newToken, refreshErr := facades.Auth(ctx).Guard(middleware.AdminGuardName).Refresh()
	if refreshErr != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "Token刷新失败",
			"error":   refreshErr.Error(),
		})
	}
	if err := setAuthenticationCookies(ctx, newToken, false); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "设置认证 Cookie 失败", err)
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "Token刷新成功",
		"data": map[string]any{
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

// CSRFToken 仅向已认证会话返回双提交 Token，供分域管理前端写入请求头。
func (r *AuthController) CSRFToken(ctx http.Context) http.Response {
	token := ctx.Request().Cookie(middleware.CSRFTokenCookieName)
	if token == "" {
		return utils.ErrorResponse(ctx, 401, "缺少 CSRF Token", "CSRF_TOKEN_MISSING")
	}
	return utils.SuccessResponse(ctx, "success", map[string]any{"csrf_token": token})
}

func (r *AuthController) Logout(ctx http.Context) http.Response {
	clearAuthenticationCookies(ctx)
	return utils.SuccessResponse(ctx, "已退出登录")
}
