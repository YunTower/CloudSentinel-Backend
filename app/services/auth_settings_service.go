package services

import (
	"strconv"
	"strings"

	"goravel/app/repositories"

	"github.com/goravel/framework/facades"
)

// 认证配置在 system_settings 中的键名。
const (
	authSettingJWTSecret  = "jwt_secret"
	authSettingJWTExpiry  = "jwt_expiration" // 秒
	authSettingSessionTTL = "session_timeout" // 秒
)

// legacyJWTSecretPlaceholder 是 seeder 写入的历史占位密钥，属于已知常量，
// 视为“未显式配置”，避免将运行时认证密钥静默降级为公开值。
const legacyJWTSecretPlaceholder = "cloudsentinel-secret-key-change-in-production"

// SyncAuthSettingsFromDB 将 system_settings 中的认证配置同步到运行时 config。
//
// 语义（P2-01 处理决策：支持动态配置而非仅环境变量）：
//   - DB 中显式配置的 jwt_secret 优先于环境变量 JWT_SECRET；seeder 占位符不算配置。
//   - config.jwt.ttl（分钟）取 session_timeout 与 jwt_expiration（均为秒）中较小值，
//     保证会话有效期不会因任一配置被放宽；两者都未配置时保留环境变量默认。
//   - 密钥轮换后旧 token 立即失效，符合“密钥轮换→令牌失效”的安全语义。
//
// 该函数应在启动时（bootstrap 之后）、登录/刷新前以及权限设置保存后调用。
func SyncAuthSettingsFromDB() {
	repo := repositories.GetSystemSettingRepository()

	if secret := strings.TrimSpace(repo.GetValue(authSettingJWTSecret, "")); secret != "" && secret != legacyJWTSecretPlaceholder {
		// Add 即 vip.Set：运行时覆盖 jwt.secret（JwtGuard 签发与校验均动态读取）
		facades.Config().Add("jwt.secret", secret)
	}

	ttlMinutes := int64(0)
	if sec := parseIntSetting(repo.GetValue(authSettingSessionTTL, "")); sec > 0 {
		ttlMinutes = sec / 60
	}
	if sec := parseIntSetting(repo.GetValue(authSettingJWTExpiry, "")); sec > 0 {
		if m := sec / 60; ttlMinutes == 0 || m < ttlMinutes {
			ttlMinutes = m
		}
	}
	if ttlMinutes > 0 {
		facades.Config().Add("jwt.ttl", ttlMinutes)
	}
}

func parseIntSetting(raw string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}
