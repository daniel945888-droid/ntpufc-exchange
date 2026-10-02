package repositories

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCacheRepo struct {
	client *redis.Client
}

func NewRedisCacheRepo(addr string, password string, db int) (*RedisCacheRepo, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// 測試連線 (Ping)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return &RedisCacheRepo{client: client}, nil
}

// ExistsOrSet 檢查鍵是否存在；若不存在則寫入 (原子操作)
func (r *RedisCacheRepo) ExistsOrSet(ctx context.Context, key string, ttlSeconds int) (bool, error) {
	// 使用 Redis 的 SETNX (Set if Not eXists) 指令
	// 如果 Key 不存在，寫入並回傳 true；如果 Key 已存在，什麼都不做並回傳 false
	success, err := r.client.SetNX(ctx, key, "1", time.Duration(ttlSeconds)*time.Second).Result()
	if err != nil {
		return false, err
	}

	// success 為 true 代表「剛剛才寫入」，也就是原本不存在 (exists = false)
	// success 為 false 代表「寫入失敗，因為已經有了」，也就是原本就存在 (exists = true)
	return !success, nil
}
