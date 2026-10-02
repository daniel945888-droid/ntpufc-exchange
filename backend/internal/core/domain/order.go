package domain

import "time"

// 定義常數，限制買賣方向與訂單狀態，防止魔術字串
type OrderSide string
type OrderStatus string

const (
	Buy  OrderSide = "buy"
	Sell OrderSide = "sell"

	Pending  OrderStatus = "pending"
	Filled   OrderStatus = "filled"
	Canceled OrderStatus = "canceled"
)

// Order 代表系統內的一筆交易委託單
type Order struct {
	ID             string
	UserID         string
	Symbol         string
	Side           OrderSide
	Qty            float64
	Price          float64
	Leverage       int
	Status         OrderStatus
	IdempotencyKey string
	CreatedAt      time.Time
	RemainingQty   float64 // ✨ 新增：未成交數量
}
