package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	_ "github.com/lib/pq" // ✨ 補上這行！這個底線(blank import)是告訴 Go 要載入 Postgres 驅動

	"ntpufc-exchange/backend/internal/adapters/engine"
	"ntpufc-exchange/backend/internal/adapters/gateway"
	"ntpufc-exchange/backend/internal/adapters/repositories"
	"ntpufc-exchange/backend/internal/core/services"
)

// ==========================================
// 🛠️ 尚未替換的假基礎設施 (Dummy Adapters)
// ==========================================
type DummyCacheRepo struct{}

// 💡 把它加回來：這是 Redis 實作前，用來騙過 Go 介面檢查的替身
func (d *DummyCacheRepo) ExistsOrSet(ctx context.Context, key string, ttlSeconds int) (bool, error) {
	return false, nil // 永遠回傳 false，代表「沒有重複發送」
}

// ==========================================
// 🚀 系統入口與組裝
// ==========================================
func main() {
	// 1. 連線基礎設施
	connStr := "postgres://exchange_admin:secretpassword@localhost:5432/exchange_db?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		panic(fmt.Sprintf("資料庫連線失敗: %v", err))
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		panic(fmt.Sprintf("無法 Ping 到資料庫，請確認 Docker 是否啟動: %v", err))
	}
	fmt.Println("✅ 成功連線至 PostgreSQL 資料庫！")

	// 2. 啟動背景風控引擎
	monitor := services.NewMarginMonitor()
	monitor.Start()
	defer monitor.Stop() // main 函數結束時才會觸發這個

	// 3. 實例化底層組件 (積木) & 啟動清算中心
	// 3. 實例化底層組件 (積木) & 啟動清算中心
	// ✨ 換上剛買的全新引擎：真正的 PostgreSQL 訂單資料庫
	orderRepo, err := repositories.NewPostgresOrderRepo(db)
	if err != nil {
		panic(fmt.Sprintf("訂單資料表初始化失敗: %v", err))
	}

	// ✨ 換上真正的 Redis！(假設你的 Docker Redis 跑在預設的 6379 port)
	cacheRepo, err := repositories.NewRedisCacheRepo("localhost:6379", "", 0)
	if err != nil {
		panic(fmt.Sprintf("Redis 連線失敗，請確認 Docker Redis 是否啟動: %v", err))
	}
	walletRepo, err := repositories.NewPostgresWalletRepo(db)
	if err != nil {
		panic(fmt.Sprintf("錢包資料表初始化失敗: %v", err))
	}

	clearingSvc := services.NewClearingService(walletRepo)
	clearingSvc.Start()
	orderBook := engine.NewInMemoryOrderBook(clearingSvc.TradeChan, nil)
	// ==========================================
	// ✨ Phase 2: 裝上遊戲大腦 (GameManager)
	// ==========================================
	// 建立 10 分鐘為一局的遊戲管理器
	// 1. 先宣告一個空的 wsPool 變數指標 (讓閉包可以先抓到它的參考)
	var wsPool *gateway.WsPool

	// 2. 建立 10 分鐘為一局的遊戲管理器
	gameManager := services.NewGameManager(10*time.Minute, func(state services.GameState, msg string) {
		fmt.Printf("📢 [全服廣播] 遊戲狀態變更為: %s - %s\n", state, msg)

		// 當未來遊戲狀態改變、回呼被觸發時，wsPool 已經出生了，可以安全使用！
		if wsPool != nil {
			// 直接將事件名稱與資料結構傳入，讓 WsPool 統一幫你轉成 JSON 廣播
			wsPool.Broadcast("game_state_sync", map[string]string{
				"state":   string(state),
				"message": msg,
			})
		}
	})

	// 3. 注入組件給業務指揮官
	orderSvc := services.NewOrderService(orderRepo, walletRepo, cacheRepo, orderBook, gameManager)

	// 4. 正式將 wsPool 實例化賦值！
	wsPool = gateway.NewWsPool(orderSvc)

	// 接通推播專線
	clearingSvc.SetNotifier(wsPool)
	orderBook.SetNotifier(wsPool)
	http.HandleFunc("/ws", wsPool.HandleConnection)
	// 👑 主持人專用路由：啟動遊戲並重置世界
	http.HandleFunc("/api/admin/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		fmt.Println("⚠️ [系統警告] 主持人按下啟動鍵，準備重置世界...")

		// 1. 🧹 記憶體驅魔：清空撮合引擎的所有排隊訂單
		orderBook.Clear()

		// 2. 💥 資料庫核彈：清空所有歷史訂單與撮合紀錄 (注意：連 match_logs 也要清掉，避免孤兒紀錄)
		if _, err := db.Exec("TRUNCATE TABLE orders, match_logs, positions;"); err != nil {
			fmt.Printf("❌ 清空歷史資料表失敗: %v\n", err)
		} else {
			fmt.Println("✅ 歷史訂單、明細、與持倉紀錄已全數刪除！")
		}

		// 3. 💰 財富重置：所有玩家的錢包餘額歸零，重發 $10,000 本金，並解鎖所有資金
		if _, err := db.Exec("UPDATE wallets SET available_balance = 10000.00, locked_balance = 0.00;"); err != nil {
			fmt.Printf("❌ 重置玩家錢包失敗: %v\n", err)
		} else {
			fmt.Println("✅ 所有玩家資金已重置為 10,000，凍結保證金歸零！")
		}

		// 4. 🚀 正式啟動遊戲大腦倒數
		if err := gameManager.StartRound(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Write([]byte("✅ 系統已重置，遊戲成功啟動！10 分鐘倒數開始！"))
	})

	// ==========================================
	// 🚀 伺服器啟動碼 (保持系統持續運作的命脈，絕對不能刪！)
	// ==========================================
	port := ":8080"
	fmt.Printf("🚀 高頻交易伺服器已啟動，正在監聽 Port %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		fmt.Println("伺服器啟動失敗:", err)
	}
}
