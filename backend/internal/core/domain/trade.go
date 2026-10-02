package domain

// TradeEvent 撮合成功後產生的成交事件
type TradeEvent struct {
	BuyerID       string
	SellerID      string
	Symbol        string
	BuyerOrderID  string // ✨ 新增：買方訂單編號
	SellerOrderID string // ✨ 新增：賣方訂單編號
	Price         float64
	Qty           float64
}
