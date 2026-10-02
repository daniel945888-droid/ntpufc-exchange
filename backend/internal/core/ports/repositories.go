package ports

import (
	"context"
	"ntpufc-exchange/backend/internal/core/domain"
)

// OrderRepository 訂單持久化合約 (PostgreSQL 實作)
type OrderRepository interface {
	Save(ctx context.Context, order *domain.Order) error
	FindByID(ctx context.Context, id string) (*domain.Order, error)
	UpdateStatus(ctx context.Context, id string, status domain.OrderStatus) error
}

// WalletRepository 資金與保證金合約 (PostgreSQL 實作)
type WalletRepository interface {
	// LockFunds 盤前風控：凍結下單保證金
	LockFunds(ctx context.Context, userID string, amount float64) error
	// ReleaseFunds 撤單或結算：解凍保證金
	ReleaseFunds(ctx context.Context, userID string, amount float64) error
	// GetBalance 取得可用與凍結餘額
	GetBalance(ctx context.Context, userID string) (available float64, locked float64, err error)
	// ✨ 補上交割清算合約
	SettleTrade(ctx context.Context, event domain.TradeEvent) error
	// ✨ 補上取得持倉數量的方法
	GetPosition(ctx context.Context, userID string, symbol string) (float64, error)
}

// IdempotencyRepository 防重放合約 (Redis 實作)
type IdempotencyRepository interface {
	// ExistsOrSet 檢查鍵是否存在；若不存在則寫入並設定過期秒數 (TTL)
	// 回傳 exists = true 表示該請求已被處理過 (拒絕重送)
	ExistsOrSet(ctx context.Context, key string, ttlSeconds int) (exists bool, err error)
}

// ✨ 新增推播介面
type NotificationPort interface {
	// 傳送通知給指定的玩家 (userID), messageType 是訊息種類 (例如 "TRADE_SUCCESS")
	SendNotification(userID string, messageType string, data interface{})
	// ✨ 新增的：對全伺服器在線玩家廣播 (用於市場報價、K線更新)
	Broadcast(messageType string, data interface{})
}
