package services

import (
	"context"
	"fmt"

	"ntpufc-exchange/backend/internal/core/domain"
	"ntpufc-exchange/backend/internal/core/ports"
)

type ClearingService struct {
	walletRepo ports.WalletRepository
	notifier   ports.NotificationPort // ✨ 新增：推播器
	TradeChan  chan domain.TradeEvent // 廣播通道
}

// NewClearingService 建立清算中心，並初始化容量為 1000 的通道緩衝
func NewClearingService(walletRepo ports.WalletRepository) *ClearingService {
	return &ClearingService{
		walletRepo: walletRepo,
		TradeChan:  make(chan domain.TradeEvent, 1000),
	}
}

// ✨ 新增這個方法：讓外部在系統組裝完畢後，再把推播專線接上來
func (s *ClearingService) SetNotifier(notifier ports.NotificationPort) {
	s.notifier = notifier
}

// Start 啟動背景工人監聽通道
func (s *ClearingService) Start() {
	go func() {
		fmt.Println("🏦 [清算中心] 背景工人已啟動，等待成交事件...")
		for event := range s.TradeChan {
			// 一旦通道內有資料，立刻呼叫資料庫執行 ACID 交割
			err := s.walletRepo.SettleTrade(context.Background(), event)
			if err != nil {
				fmt.Printf("❌ [清算中心] 交割失敗 (買方:%s 賣方:%s): %v\n", event.BuyerID, event.SellerID, err)
			} else {
				// ✨ 交割成功！檢查是否有接上推播線，有的話就發送結構化喜訊！
				if s.notifier != nil {
					// 1. 打包給【買方】的專屬 JSON 資訊
					buyerData := map[string]interface{}{
						"action":  "filled",
						"symbol":  event.Symbol,
						"side":    "buy",
						"price":   event.Price,
						"qty":     event.Qty,
						"cost":    event.Price * event.Qty, // 花費的總金額
						"message": fmt.Sprintf("買單成交：%s 以 $%.2f 買入 %.0f 張", event.Symbol, event.Price, event.Qty),
					}
					s.notifier.SendNotification(event.BuyerID, "TRADE_SUCCESS", buyerData)

					// 2. 打包給【賣方】的專屬 JSON 資訊
					sellerData := map[string]interface{}{
						"action":  "filled",
						"symbol":  event.Symbol,
						"side":    "sell",
						"price":   event.Price,
						"qty":     event.Qty,
						"revenue": event.Price * event.Qty, // 獲得的總金額
						"message": fmt.Sprintf("賣單成交：%s 以 $%.2f 賣出 %.0f 張", event.Symbol, event.Price, event.Qty),
					}
					s.notifier.SendNotification(event.SellerID, "TRADE_SUCCESS", sellerData)
				}
			}
		}
	}()
}
