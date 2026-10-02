package engine

import (
	"container/heap"
	"fmt" // ✨ 新增：用來幫深度報價排序
	"sort"
	"sync"

	"ntpufc-exchange/backend/internal/core/domain"
	"ntpufc-exchange/backend/internal/core/ports" // ✨ 新增：為了使用 NotificationPort
)

// OrderHeap 提供基礎的 Heap 實作
type OrderHeap []*domain.Order

func (h OrderHeap) Len() int      { return len(h) }
func (h OrderHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *OrderHeap) Push(x any)   { *h = append(*h, x.(*domain.Order)) }
func (h *OrderHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return item
}

// BuyHeap: Max-Heap (買單：價格越高越優先，價格相同則時間早優先)
type BuyHeap struct{ OrderHeap }

func (h BuyHeap) Less(i, j int) bool {
	if h.OrderHeap[i].Price == h.OrderHeap[j].Price {
		return h.OrderHeap[i].CreatedAt.Before(h.OrderHeap[j].CreatedAt)
	}
	return h.OrderHeap[i].Price > h.OrderHeap[j].Price
}

// SellHeap: Min-Heap (賣單：價格越低越優先，價格相同則時間早優先)
type SellHeap struct{ OrderHeap }

func (h SellHeap) Less(i, j int) bool {
	if h.OrderHeap[i].Price == h.OrderHeap[j].Price {
		return h.OrderHeap[i].CreatedAt.Before(h.OrderHeap[j].CreatedAt)
	}
	return h.OrderHeap[i].Price < h.OrderHeap[j].Price
}

// InMemoryOrderBook 實作了 core/ports 中的 MatchingEngine 介面
type InMemoryOrderBook struct {
	buyHeap   *BuyHeap
	sellHeap  *SellHeap
	mu        sync.Mutex
	tradeChan chan<- domain.TradeEvent // ✨ 新增這行：只能發送的通道
	notifier  ports.NotificationPort   // ✨ 新增：引擎自己的大聲公
}

func NewInMemoryOrderBook(tradeChan chan<- domain.TradeEvent, notifier ports.NotificationPort) *InMemoryOrderBook {
	ob := &InMemoryOrderBook{
		buyHeap:   &BuyHeap{OrderHeap: make([]*domain.Order, 0)},
		sellHeap:  &SellHeap{OrderHeap: make([]*domain.Order, 0)},
		tradeChan: tradeChan,
		notifier:  notifier, // ✨ 裝備大聲公
	}
	heap.Init(ob.buyHeap)
	heap.Init(ob.sellHeap)
	return ob
}

func (ob *InMemoryOrderBook) PlaceOrder(order *domain.Order) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	fmt.Printf("⚙️ [引擎入口] 收到 %s 單，準備進行撮合！\n", order.Side)

	// 核心路由與排隊邏輯 (剛剛可能不見了的就是這段！)
	if order.Side == domain.Buy {
		// 1. 先去跟賣單簿對撞
		ob.matchBuy(order)
		// 2. 如果沒對撞完 (還有剩)，就存進買單簿排隊
		if order.Qty > 0 {
			heap.Push(ob.buyHeap, order)
			fmt.Printf("📥 [引擎入口] 買單未完全成交，剩餘 %.2f 張存入掛單簿 (目前買單簿總數: %d)\n", order.Qty, ob.buyHeap.Len())
		}
	} else {
		// 1. 先去跟買單簿對撞
		ob.matchSell(order)
		// 2. 如果沒對撞完 (還有剩)，就存進賣單簿排隊
		if order.Qty > 0 {
			heap.Push(ob.sellHeap, order)
			fmt.Printf("📥 [引擎入口] 賣單未完全成交，剩餘 %.2f 張存入掛單簿 (目前賣單簿總數: %d)\n", order.Qty, ob.sellHeap.Len())
		}
	}
	// ✨ 核心升級：只要有人下單（無論成交與否），就拍一張最新的深度快照廣播給所有人！
	ob.broadcastDepth(order.Symbol)
	return nil
}

// matchBuy 處理買單撮合
func (ob *InMemoryOrderBook) matchBuy(buyOrder *domain.Order) {
	for buyOrder.Qty > 0 && ob.sellHeap.Len() > 0 {
		bestSell := ob.sellHeap.OrderHeap[0]
		if buyOrder.Price < bestSell.Price {
			break // 買價低於最便宜的賣價，無法成交
		}

		// 決定成交數量 (取兩者較小值)
		matchQty := buyOrder.Qty
		if bestSell.Qty < matchQty {
			matchQty = bestSell.Qty
		}

		// 決定成交價格 (以先掛單的 Maker 價格為主)
		matchPrice := bestSell.Price

		// ✨ 發送成交事件給清算中心
		ob.tradeChan <- domain.TradeEvent{
			BuyerID:       buyOrder.UserID,
			SellerID:      bestSell.UserID,
			BuyerOrderID:  buyOrder.ID, // ✨ 補上 ID
			SellerOrderID: bestSell.ID, // ✨ 補上 ID
			Symbol:        buyOrder.Symbol,
			Price:         matchPrice,
			Qty:           matchQty,
		}

		fmt.Printf("[⚡ 撮合成功] 買方 %s 與 賣方 %s 在價格 %.2f 成交了 %.0f 張\n", buyOrder.UserID, bestSell.UserID, matchPrice, matchQty)

		// 扣除數量
		buyOrder.Qty -= matchQty
		bestSell.Qty -= matchQty

		// 如果賣單被吃光，從 Heap 中移除
		if bestSell.Qty == 0 {
			heap.Pop(ob.sellHeap)
		}
	}
}

// matchSell 處理賣單撮合
func (ob *InMemoryOrderBook) matchSell(sellOrder *domain.Order) {
	for sellOrder.Qty > 0 && ob.buyHeap.Len() > 0 {
		bestBuy := ob.buyHeap.OrderHeap[0]
		if sellOrder.Price > bestBuy.Price {
			break // 賣價高於最貴的買價，無法成交
		}

		// 決定成交數量 (取兩者較小值)
		matchQty := sellOrder.Qty
		if bestBuy.Qty < matchQty {
			matchQty = bestBuy.Qty
		}

		// 決定成交價格 (以先掛單的 Maker 價格為主)
		matchPrice := bestBuy.Price
		fmt.Println("🔍 [引擎診斷] 準備發送成交事件至 Channel...") // ✨ 加入這行
		// ✨ 發送成交事件給清算中心
		// ✨ 發送成交事件給清算中心
		ob.tradeChan <- domain.TradeEvent{
			BuyerID:       bestBuy.UserID,
			SellerID:      sellOrder.UserID,
			BuyerOrderID:  bestBuy.ID,   // ✨ 買單是從簿子裡抓出來的 bestBuy
			SellerOrderID: sellOrder.ID, // ✨ 賣單是剛剛送進來的 sellOrder
			Symbol:        sellOrder.Symbol,
			Price:         matchPrice,
			Qty:           matchQty,
		}
		fmt.Println("✅ [引擎診斷] 成功放入 Channel，準備印出撮合成功！") // ✨ 加入這行
		fmt.Printf("[⚡ 撮合成功] 買方 %s 與 賣方 %s 在價格 %.2f 成交了 %.0f 張\n", bestBuy.UserID, sellOrder.UserID, matchPrice, matchQty)

		// 扣除數量
		sellOrder.Qty -= matchQty
		bestBuy.Qty -= matchQty

		// 如果買單被吃光，從 Heap 中移除
		if bestBuy.Qty == 0 {
			heap.Pop(ob.buyHeap)
		}
	}
}

func (ob *InMemoryOrderBook) CancelOrder(orderID string) error {
	return nil // 試跑階段先略過取消邏輯
}

// ==========================================
// ✨ 市場深度 (Market Depth) 快照與廣播邏輯
// ==========================================

// DepthLevel 代表一個價格檔位上的總需求/供給
type DepthLevel struct {
	Price float64 `json:"price"`
	Qty   float64 `json:"qty"`
}

// broadcastDepth 擷取掛單簿現況並廣播給全服玩家
func (ob *InMemoryOrderBook) broadcastDepth(symbol string) {
	// 因為此函數會在 PlaceOrder 內部被呼叫，此時已持有 mu 鎖，所以這邊不需再上鎖

	// 1. 彙整買單深度 (相同價格的訂單數量要加總)
	buyLevels := make(map[float64]float64)
	for _, order := range ob.buyHeap.OrderHeap {
		buyLevels[order.Price] += order.Qty
	}

	// 2. 彙整賣單深度
	sellLevels := make(map[float64]float64)
	for _, order := range ob.sellHeap.OrderHeap {
		sellLevels[order.Price] += order.Qty
	}

	// 3. 轉換為陣列並嚴格排序 (買方由高至低，賣方由低至高)
	var buys []DepthLevel
	for p, q := range buyLevels {
		buys = append(buys, DepthLevel{Price: p, Qty: q})
	}
	sort.Slice(buys, func(i, j int) bool { return buys[i].Price > buys[j].Price })

	var sells []DepthLevel
	for p, q := range sellLevels {
		sells = append(sells, DepthLevel{Price: p, Qty: q})
	}
	sort.Slice(sells, func(i, j int) bool { return sells[i].Price < sells[j].Price })

	// 4. 只取前 5 檔 (Top 5)
	if len(buys) > 5 {
		buys = buys[:5]
	}
	if len(sells) > 5 {
		sells = sells[:5]
	}

	// 5. 打包 JSON 並透過大聲公發送全服廣播！
	depthData := map[string]interface{}{
		"symbol": symbol,
		"bids":   buys,  // 買盤 (買方喊價)
		"asks":   sells, // 賣盤 (賣方喊價)
	}

	if ob.notifier != nil {
		ob.notifier.Broadcast("MARKET_DEPTH", depthData)
	}

}

// 🧹 驅魔專用：清空所有掛單與深度
func (ob *InMemoryOrderBook) Clear() {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	// 無情地把買賣陣列重置為空
	ob.buyHeap = &BuyHeap{OrderHeap: make([]*domain.Order, 0)}
	ob.sellHeap = &SellHeap{OrderHeap: make([]*domain.Order, 0)}
	heap.Init(ob.buyHeap)
	heap.Init(ob.sellHeap)

	fmt.Println("🧹 [撮合引擎] 驅魔完成：掛單簿已全面清空！")

	// 拍一張「空空如也」的深度快照，並廣播給前端，讓畫面上的買賣五檔歸零
	// 這裡假設你的遊戲只有 "AI_STOCK" 這個商品，或者你也可以迴圈清空所有商品的深度
	ob.broadcastDepth("AI_STOCK")
}
func (ob *InMemoryOrderBook) SetNotifier(notifier ports.NotificationPort) {
	ob.notifier = notifier
}
