package middleware

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"goravel/app/facades"
)

// JWT 吊销黑名单。
//
// 登出/改密后将当前令牌加入黑名单，TTL 等于令牌剩余有效期；认证中间件
// 统一校验。黑名单基于缓存实现：使用内存/文件缓存驱动时，进程重启会
// 清空黑名单（令牌剩余有效期通常较短，风险可接受）；需要跨重启吊销时
// 请配置持久化缓存驱动。

const jwtBlacklistKeyPrefix = "jwt_blacklist:"

func tokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// parseTokenExpiry 从 JWT payload 中读取 exp（不验签：仅用于计算黑名单 TTL，
// 不影响认证决策本身）。
func parseTokenExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 && len(parts) != 3 {
		return time.Time{}, false
	}
	payload := parts[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		// 兼容带 padding 的编码
		decoded, err = base64.URLEncoding.DecodeString(payload)
		if err != nil {
			return time.Time{}, false
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

// BlacklistToken 吊销令牌直至其自然过期。
func BlacklistToken(token string) {
	if token == "" {
		return
	}
	exp, ok := parseTokenExpiry(token)
	if !ok || !time.Now().Before(exp) {
		// 无 exp 或已过期：无需吊销
		return
	}
	ttl := time.Until(exp)
	_ = facades.Cache().Put(jwtBlacklistKeyPrefix+tokenFingerprint(token), true, ttl)
}

// IsTokenBlacklisted 检查令牌是否已被吊销。
func IsTokenBlacklisted(token string) bool {
	if token == "" {
		return false
	}
	return facades.Cache().Get(jwtBlacklistKeyPrefix+tokenFingerprint(token)) != nil
}
