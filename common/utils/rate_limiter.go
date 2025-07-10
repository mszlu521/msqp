package utils

import (
	"sync"
	"time"
)

// RateLimiter 实现令牌桶算法的限流器
type RateLimiter struct {
	rate       float64    // 每秒允许的请求数
	capacity   float64    // 桶的容量
	tokens     float64    // 当前令牌数
	lastRefill time.Time  // 上次填充令牌的时间
	mu         sync.Mutex // 互斥锁，保护并发访问
}

// NewRateLimiter 创建一个新的限流器
// rate: 每秒允许的请求数
// burst: 允许的突发请求数（桶的容量）
func NewRateLimiter(rate int, burst int) *RateLimiter {
	return &RateLimiter{
		rate:       float64(rate),
		capacity:   float64(burst * rate), // 桶的容量 = 突发请求数 * 每秒请求数
		tokens:     float64(burst * rate), // 初始令牌数 = 桶的容量
		lastRefill: time.Now(),
	}
}

// Allow 判断当前请求是否允许通过
// 返回 true 表示允许，false 表示拒绝
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 计算从上次填充到现在经过的时间
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()

	// 计算应该添加的令牌数
	newTokens := elapsed * rl.rate

	// 更新令牌数，不超过桶的容量
	if rl.tokens < rl.capacity {
		rl.tokens = min(rl.capacity, rl.tokens+newTokens)
	}

	// 更新上次填充时间
	rl.lastRefill = now

	// 如果有足够的令牌，则允许请求通过
	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return true
	}

	return false
}

// min 返回两个浮点数中的较小值
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// SetRate 设置限流器的速率
func (rl *RateLimiter) SetRate(rate int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.rate = float64(rate)
}

// SetBurst 设置限流器的突发请求数
func (rl *RateLimiter) SetBurst(burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	newCapacity := float64(burst) * rl.rate
	// 如果新容量大于当前容量，则增加令牌数
	if newCapacity > rl.capacity {
		rl.tokens += (newCapacity - rl.capacity)
	}
	rl.capacity = newCapacity
}

// GetRate 获取限流器的速率
func (rl *RateLimiter) GetRate() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	return int(rl.rate)
}

// GetBurst 获取限流器的突发请求数
func (rl *RateLimiter) GetBurst() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	return int(rl.capacity / rl.rate)
}
