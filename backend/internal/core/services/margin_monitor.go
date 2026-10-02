package services

import (
	"fmt"
	"time"
)

// MarginMonitor 負責背景監控保證金維持率
type MarginMonitor struct {
	// 未來這裡會注入 PositionRepository (取得玩家部位) 與 MatchingEngine (發送強制平倉單)
	stop chan struct{}
}

func NewMarginMonitor() *MarginMonitor {
	return &MarginMonitor{
		stop: make(chan struct{}), // 用來接收停止訊號的 Channel
	}
}

// Start 啟動背景監控迴圈 (非阻塞)
func (m *MarginMonitor) Start() {
	// 開啟一個獨立的 Goroutine，它會在背景默默跑，不會卡住主程式
	go func() {
		// 設定一個計時器：每 3 秒掃描一次全市場玩家 (實戰中可調為 500 毫秒)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		fmt.Println("🛡️ [即時風控引擎] 已啟動，開始背景盯盤...")

		for {
			select {
			case <-ticker.C:
				// 時間到了，執行掃描
				m.scanAndLiquidate()
			case <-m.stop:
				// 收到關閉伺服器的訊號，安全退出迴圈
				fmt.Println("🛡️ [即時風控引擎] 收到關閉訊號，停止監控。")
				return
			}
		}
	}()
}

// Stop 安全地停止背景監控
func (m *MarginMonitor) Stop() {
	close(m.stop) // 關閉 Channel，觸發上面的 <-m.stop 事件
}

func (m *MarginMonitor) scanAndLiquidate() {
	// 這裡保留骨架，未來會在這裡實作真實的數學公式：
	// 1. 取得所有玩家目前持倉部位
	// 2. 取得市場最新成交價
	// 3. 計算維持率 = (帳戶餘額 + 未實現損益) / 原始保證金
	// 4. 若維持率 < 閾值，自動產生一張「市價平倉單」並丟給 MatchingEngine

	// 試跑階段先印出心跳日誌，證明 Goroutine 有在運作
	// fmt.Println("🛡️ [即時風控引擎] 掃描部位維持率正常...")
}
