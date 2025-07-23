package auth

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter with improved precision
type RateLimiter struct {
	mu           sync.Mutex
	rate         float64       // tokens per second
	capacity     int64         // maximum token capacity
	tokens       float64       // current token count
	last         time.Time     // last time tokens were added
	reservations chan struct{} // channel for token reservations
}

// NewRateLimiter creates a new rate limiter with specified rate and interval
// rate: number of allowed operations
// interval: time period for the rate
func NewRateLimiter(rate int64, interval time.Duration) *RateLimiter {
	if rate <= 0 || interval <= 0 {
		panic("rate and interval must be positive")
	}

	// Calculate tokens per second
	tokensPerSecond := float64(rate) / interval.Seconds()

	return &RateLimiter{
		rate:         tokensPerSecond,
		capacity:     rate,
		tokens:       float64(rate), // Start with full capacity
		last:         time.Now(),
		reservations: make(chan struct{}, rate), // Buffer size matches capacity
	}
}

// Allow checks if a request is allowed under the rate limit
// Returns true if allowed, false if rate limited
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Calculate elapsed time since last token refresh with nanosecond precision
	elapsed := now.Sub(rl.last).Seconds()
	rl.last = now

	// Add tokens based on elapsed time with proper rounding
	newTokens := elapsed * rl.rate
	rl.tokens = min(float64(rl.capacity), rl.tokens+newTokens)

	// Check if we have enough tokens
	if rl.tokens < 1.0 {
		return false
	}

	// Consume one token
	rl.tokens--

	// Add reservation for monitoring purposes
	select {
	case rl.reservations <- struct{}{}:
		// Successfully reserved
	default:
		// Channel full, clean up old reservations
		select {
		case <-rl.reservations:
			rl.reservations <- struct{}{}
		default:
			// This shouldn't happen with proper sizing
		}
	}

	return true
}

// WaitTime returns the time to wait before the next token is available
func (rl *RateLimiter) WaitTime() time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.tokens >= 1.0 {
		return 0
	}

	// Calculate time needed for one more token
	tokensNeeded := 1.0 - rl.tokens
	secondsNeeded := tokensNeeded / rl.rate

	return time.Duration(secondsNeeded * float64(time.Second))
}

// TryUntil tries to acquire permission until the given deadline
// Returns true if permission was acquired, false if deadline exceeded
func (rl *RateLimiter) TryUntil(deadline time.Time) bool {
	for {
		if rl.Allow() {
			return true
		}

		waitTime := rl.WaitTime()
		sleepUntil := time.Now().Add(waitTime)

		if sleepUntil.After(deadline) {
			return false
		}

		time.Sleep(waitTime)
	}
}

// Reset resets the rate limiter to its initial state
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.tokens = float64(rl.capacity)
	rl.last = time.Now()

	// Clear reservations
	for {
		select {
		case <-rl.reservations:
			// Remove reservation
		default:
			return
		}
	}
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
