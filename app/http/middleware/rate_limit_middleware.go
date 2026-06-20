package middleware

import (
	"sync"
	"time"

	"github.com/goravel/framework/contracts/http"
)

type rateLimitBucket struct {
	count       int
	windowStart time.Time
}

var (
	publicRateLimitMu      sync.Mutex
	publicRateLimitBuckets = map[string]*rateLimitBucket{}
)

func PublicRateLimit(maxRequests int, window time.Duration) http.Middleware {
	if maxRequests <= 0 {
		maxRequests = 120
	}
	if window <= 0 {
		window = time.Minute
	}

	return func(ctx http.Context) {
		key := ctx.Request().Ip()
		if key == "" {
			key = "unknown"
		}
		now := time.Now()

		publicRateLimitMu.Lock()
		bucket := publicRateLimitBuckets[key]
		if bucket == nil || now.Sub(bucket.windowStart) >= window {
			bucket = &rateLimitBucket{windowStart: now}
			publicRateLimitBuckets[key] = bucket
		}
		bucket.count++
		allowed := bucket.count <= maxRequests
		if len(publicRateLimitBuckets) > 4096 {
			for ip, item := range publicRateLimitBuckets {
				if now.Sub(item.windowStart) >= window {
					delete(publicRateLimitBuckets, ip)
				}
			}
		}
		publicRateLimitMu.Unlock()

		if !allowed {
			ctx.Response().Json(429, map[string]interface{}{
				"status": false, "message": "请求过于频繁，请稍后再试",
			})
			return
		}
		ctx.Request().Next()
	}
}
