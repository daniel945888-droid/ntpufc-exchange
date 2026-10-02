package gateway

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"ntpufc-exchange/backend/internal/core/domain"
	"ntpufc-exchange/backend/internal/core/services"

	"github.com/gorilla/websocket"
)

type WsOrderPayload struct {
	Action   string  `json:"action"`
	Symbol   string  `json:"symbol"`
	Side     string  `json:"side"`
	Price    float64 `json:"price"`
	Qty      float64 `json:"qty"`
	Leverage int     `json:"leverage"`
	OrderID  string  `json:"order_id"`
}

type Client struct {
	ID     string
	Conn   *websocket.Conn
	Bucket *TokenBucket
}

type WsPool struct {
	clients     map[string]*Client
	mu          sync.RWMutex
	upgrader    websocket.Upgrader
	orderSvc    *services.OrderService
	GameStatus  string // ✨ 新增：閘道器自己記憶目前的遊戲狀態 (給遲到的玩家用)
	GameEndTime int64  // ✨ 新增：記憶遊戲結束的絕對時間戳 (毫秒)
}

func NewWsPool(orderSvc *services.OrderService) *WsPool {
	return &WsPool{
		clients:    make(map[string]*Client),
		orderSvc:   orderSvc,
		GameStatus: "WAITING", // ✨ 預設狀態
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (pool *WsPool) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := pool.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("升級 WebSocket 失敗:", err)
		return
	}

	// 🚨 解決核彈級 Bug：透過 Proxy 進來的連線不能用 r.RemoteAddr
	// 必須抓取前端帶來的 user_id 參數
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		// 防呆機制：如果真的沒帶，給他一個隨機亂數，避免所有人撞號
		userID = fmt.Sprintf("Guest-%d", time.Now().UnixMicro())
	}

	client := &Client{
		ID:     userID,
		Conn:   conn,
		Bucket: NewTokenBucket(5, 5),
	}

	pool.mu.Lock()
	pool.clients[userID] = client
	pool.mu.Unlock()

	fmt.Printf("[登入] 玩家 %s 連線成功。當前在線人數: %d\n", userID, pool.OnlineCount())
	go pool.listenToClient(client)
}

func (pool *WsPool) listenToClient(client *Client) {
	defer func() {
		pool.mu.Lock()
		delete(pool.clients, client.ID)
		pool.mu.Unlock()
		client.Conn.Close()
		fmt.Printf("[登出] 玩家 %s 斷線。\n", client.ID)
	}()

	for {
		var payload WsOrderPayload
		err := client.Conn.ReadJSON(&payload)
		if err != nil {
			fmt.Printf("讀取玩家 %s 訊息失敗或斷線: %v\n", client.ID, err)
			break
		}

		if !client.Bucket.Allow() {
			client.Conn.WriteJSON(map[string]string{"error": "系統警告：你點太快了！觸發頻率限制"})
			continue
		}

		switch payload.Action {
		// ✨ 新增：回答遲到玩家的狀態查詢
		case "get_status":
			pool.mu.RLock()
			currentState := pool.GameStatus
			currentEndTime := pool.GameEndTime // ✨ 把時間拿出來
			pool.mu.RUnlock()

			client.Conn.WriteJSON(map[string]interface{}{
				"type": "game_state_sync",
				"message": map[string]interface{}{
					"state":    currentState,
					"message":  "同步當前遊戲狀態",
					"end_time": currentEndTime,
				},
			})

		case "place_order":
			order := &domain.Order{
				ID:       fmt.Sprintf("ORD-%d", time.Now().UnixNano()),
				UserID:   client.ID,
				Symbol:   payload.Symbol,
				Side:     domain.OrderSide(payload.Side),
				Price:    payload.Price,
				Qty:      payload.Qty,
				Leverage: payload.Leverage,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = pool.orderSvc.CreateOrder(ctx, order)

			if err != nil {
				client.Conn.WriteJSON(map[string]string{"error": err.Error()})
			} else {
				client.Conn.WriteJSON(map[string]interface{}{
					"type":     "ORDER_ACK",
					"order_id": order.ID,
					"side":     string(order.Side),
					"price":    order.Price,
					"qty":      order.Qty,
					"message":  "委託單已送出並進入撮合流程",
				})
			}
			cancel()

		case "cancel_order":
			if payload.OrderID == "" {
				client.Conn.WriteJSON(map[string]string{"error": "撤單失敗：未提供訂單 ID"})
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = pool.orderSvc.CancelOrder(ctx, payload.OrderID, client.ID)

			if err != nil {
				client.Conn.WriteJSON(map[string]string{"error": err.Error()})
			} else {
				client.Conn.WriteJSON(map[string]interface{}{
					"type":     "CANCEL_ACK",
					"order_id": payload.OrderID,
					"message":  fmt.Sprintf("✅ 訂單 %s 撤銷成功，已退還保證金", payload.OrderID),
				})
			}
			cancel()

		default:
			client.Conn.WriteJSON(map[string]string{"error": "未知的操作指令"})
		}
	}
}

func (pool *WsPool) OnlineCount() int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	return len(pool.clients)
}

func (p *WsPool) SendNotification(userID string, messageType string, data interface{}) {
	p.mu.RLock()
	client, exists := p.clients[userID]
	p.mu.RUnlock()

	if exists {
		// ✨ 順應你的原有設計：如果是 map，直接把 type 塞進去，保持 JSON 扁平！
		if payload, ok := data.(map[string]interface{}); ok {
			payload["type"] = messageType
			client.Conn.WriteJSON(payload)
		} else {
			// 備用方案 (相容 game_state_sync)
			client.Conn.WriteJSON(map[string]interface{}{
				"type":    messageType,
				"message": data,
			})
		}
	}
}
func (pool *WsPool) Broadcast(messageType string, data interface{}) {
	var overrideMsg map[string]interface{}

	if messageType == "game_state_sync" {
		if stateData, ok := data.(map[string]string); ok {
			pool.mu.Lock()
			stateStr := stateData["state"]

			// ✨ 如果是剛切換到 PLAYING，記錄結束時間 (當下時間 + 10分鐘 = 600,000毫秒)
			if stateStr == "PLAYING" && pool.GameStatus != "PLAYING" {
				pool.GameEndTime = time.Now().UnixMilli() + 600000
			} else if stateStr != "PLAYING" {
				pool.GameEndTime = 0 // 重置
			}

			pool.GameStatus = stateStr
			currentEndTime := pool.GameEndTime
			pool.mu.Unlock()

			// ✨ 重新打包，把 end_time 塞進去給所有前端
			overrideMsg = map[string]interface{}{
				"type": messageType,
				"message": map[string]interface{}{
					"state":    stateStr,
					"message":  stateData["message"],
					"end_time": currentEndTime,
				},
			}
			fmt.Printf("🔍 [閘道器對時] 狀態: %s, 結束時間戳: %d\n", stateStr, currentEndTime)
		}
	}

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	// 決定要發送原始資料，還是我們加了時間的資料
	var finalMsg interface{}
	if overrideMsg != nil {
		finalMsg = overrideMsg
	} else {
		finalMsg = map[string]interface{}{
			"type":    messageType,
			"message": data,
		}
	}

	for _, client := range pool.clients {
		client.Conn.WriteJSON(finalMsg)
	}
}
