package middleware

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/disgoorg/bot-template/backend/utils"
)

// RateLimiter implements a simple in-memory rate limiter
type RateLimiter struct {
	requests map[string][]time.Time
	mutex    sync.RWMutex
	window   time.Duration
	limit    int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		window:   window,
		limit:    limit,
	}

	// Cleanup old entries every minute
	go rl.cleanup()

	return rl
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Get existing requests for this key
	requests := rl.requests[key]

	// Remove old requests
	var validRequests []time.Time
	for _, req := range requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}

	// Check if we're under the limit
	if len(validRequests) >= rl.limit {
		// Update the stored requests (without adding new one)
		rl.requests[key] = validRequests
		return false
	}

	// Add this request and allow it
	validRequests = append(validRequests, now)
	rl.requests[key] = validRequests
	return true
}

// cleanup removes old entries from the rate limiter
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mutex.Lock()
		cutoff := time.Now().Add(-rl.window * 2) // Keep some buffer

		for key, requests := range rl.requests {
			var validRequests []time.Time
			for _, req := range requests {
				if req.After(cutoff) {
					validRequests = append(validRequests, req)
				}
			}

			if len(validRequests) == 0 {
				delete(rl.requests, key)
			} else {
				rl.requests[key] = validRequests
			}
		}
		rl.mutex.Unlock()
	}
}

// RateLimit middleware limits requests per IP address
func RateLimit(limit int, window time.Duration) fiber.Handler {
	limiter := NewRateLimiter(limit, window)

	return func(c *fiber.Ctx) error {
		ip := utils.GetIPAddress(c)
		
		if !limiter.Allow(ip) {
			slog.Warn("Rate limit exceeded",
				slog.String("ip", ip),
				slog.String("path", c.Path()),
				slog.String("method", c.Method()),
				slog.Int("limit", limit),
				slog.Duration("window", window))

			return utils.SendError(c, 429, "RATE_LIMIT_EXCEEDED", 
				"Too many requests. Please try again later.", nil)
		}

		return c.Next()
	}
}

// AuthRateLimit middleware limits authentication attempts
func AuthRateLimit() fiber.Handler {
	// More restrictive rate limiting for auth endpoints
	return RateLimit(5, time.Minute)
}

// APIRateLimit middleware limits API requests
func APIRateLimit() fiber.Handler {
	// Standard API rate limiting
	return RateLimit(100, time.Minute)
}

// UploadRateLimit middleware limits file upload requests
func UploadRateLimit() fiber.Handler {
	// Very restrictive rate limiting for uploads
	return RateLimit(10, time.Hour)
}