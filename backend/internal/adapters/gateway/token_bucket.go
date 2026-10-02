package gateway

import (
	"sync"
	"time"
)

// TokenBucket 實作 API 節流防護
type TokenBucket struct {
	rate       float64    // 每秒補充的令牌數 (例如：5.0)
	capacity   float64    // 桶子的最大容量 (例如：5.0)
	tokens     float64    // 目前桶子裡剩下的令牌數
	lastRefill time.Time  // 上次補充令牌的時間
	mu         sync.Mutex // 互斥鎖，確保高併發下扣除令牌不會發生競態條件 (Race Condition)
}

func NewTokenBucket(rate, capacity float64) *TokenBucket {
	return &TokenBucket{
		rate:       rate,
		capacity:   capacity,
		tokens:     capacity,
		lastRefill: time.Now(),
	}
}

// Allow 判斷這次請求是否放行
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	// 根據時間差，計算這段時間內應該補充多少令牌
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.rate

	// 確保令牌數不超過桶子最大容量
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastRefill = now

	// 如果桶子裡有至少 1 個令牌，扣除 1 個並放行
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}

	// 令牌不夠，阻斷請求
	return false
}
