package auth

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	rate     float64
	capacity int64
	tokens   float64
	last     time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int64, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		rate:     float64(rate) / interval.Seconds(),
		capacity: rate,
		tokens:   float64(rate),
		last:     time.Now(),
	}
}

// Allow checks if a request is allowed under the rate limit
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.last).Seconds()
	rl.last = now

	// Add tokens based on elapsed time
	rl.tokens += elapsed * rl.rate
	if rl.tokens > float64(rl.capacity) {
		rl.tokens = float64(rl.capacity)
	}

	// Check if we have enough tokens
	if rl.tokens < 1 {
		return false
	}

	rl.tokens--
	return true
}
