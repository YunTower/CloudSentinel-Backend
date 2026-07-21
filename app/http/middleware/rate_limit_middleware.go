package middleware

import (
	"sync/atomic"
	"time"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

const rateLimitGlobalKey = "__global__"

var rateLimitCleanupCounter atomic.Uint64

// consumeRateLimit performs one conditional UPSERT. SQLite serializes writes,
// so the returned affected-row count is an atomic allow/deny decision even
// when multiple local backend processes share the same database file.
func consumeRateLimit(scope, clientKey string, maxRequests int, window time.Duration, now time.Time) (bool, error) {
	windowStart := now.UTC().Truncate(window).Unix()
	result, err := facades.Orm().Query().Exec(`
		INSERT INTO rate_limit_buckets (scope, client_key, window_start, request_count, updated_at)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(scope, client_key) DO UPDATE SET
			window_start = excluded.window_start,
			request_count = CASE
				WHEN rate_limit_buckets.window_start = excluded.window_start THEN rate_limit_buckets.request_count + 1
				ELSE 1
			END,
			updated_at = excluded.updated_at
		WHERE rate_limit_buckets.window_start <> excluded.window_start
			OR rate_limit_buckets.request_count < ?`,
		scope, clientKey, windowStart, now.UTC(), maxRequests,
	)
	if err != nil {
		return false, err
	}
	return result.RowsAffected == 1, nil
}

func pruneRateLimitBuckets(now time.Time, window time.Duration) {
	if rateLimitCleanupCounter.Add(1)%256 != 0 {
		return
	}

	// Best-effort cleanup keeps the lightweight SQLite table bounded in normal
	// use. Failure does not affect the current request's protection decision.
	_, _ = facades.Orm().Query().Exec(
		"DELETE FROM rate_limit_buckets WHERE updated_at < ?",
		now.UTC().Add(-2*window),
	)
}

func RateLimit(scope string, maxRequests int, window time.Duration) http.Middleware {
	if maxRequests <= 0 {
		maxRequests = 120
	}
	if window <= 0 {
		window = time.Minute
	}

	return func(ctx http.Context) {
		clientKey := ctx.Request().Ip()
		if clientKey == "" {
			clientKey = "unknown"
		}
		now := time.Now()

		// Limit a single source first, then the whole route scope. The scope
		// ceiling prevents a distributed flood from bypassing per-IP limits.
		allowed, err := consumeRateLimit(scope, clientKey, maxRequests, window, now)
		if err == nil && allowed {
			allowed, err = consumeRateLimit(scope, rateLimitGlobalKey, maxRequests*12, window, now)
		}
		if err != nil {
			facades.Log().Warningf("限流计数失败，拒绝请求: %v", err)
			ctx.Response().Json(503, map[string]interface{}{
				"status": false, "message": "服务繁忙，请稍后再试",
			})
			return
		}

		pruneRateLimitBuckets(now, window)

		if !allowed {
			ctx.Response().Json(429, map[string]interface{}{
				"status": false, "message": "请求过于频繁，请稍后再试",
			})
			return
		}
		ctx.Request().Next()
	}
}

func PublicRateLimit(maxRequests int, window time.Duration) http.Middleware {
	return RateLimit("public", maxRequests, window)
}

func LoginRateLimit() http.Middleware { return RateLimit("login", 10, time.Minute) }

func AgentRateLimit() http.Middleware { return RateLimit("agent", 120, time.Minute) }

func WebSocketRateLimit() http.Middleware { return RateLimit("websocket", 20, time.Minute) }
