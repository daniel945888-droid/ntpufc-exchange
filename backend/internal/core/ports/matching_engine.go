package ports

import "ntpufc-exchange/backend/internal/core/domain"

type MatchingEngine interface {
	PlaceOrder(order *domain.Order) error
	CancelOrder(orderID string) error
	Clear() // ✨ 補上這行驅魔術
}
