package services

import (
	"context"
	"errors"
	"fmt"
	"ntpufc-exchange/backend/internal/core/domain"
	"ntpufc-exchange/backend/internal/core/ports"
)

var (
	ErrInvalidOrderParam = errors.New("合規錯誤：價格與數量必須大於 0")
	ErrDuplicateOrder    = errors.New("合規錯誤：請勿重複發送相同訂單 (防重放攔截)")
	ErrInsufficientFund  = errors.New("風控錯誤：可用保證金不足")
)

type OrderService struct {
	orderRepo   ports.OrderRepository
	walletRepo  ports.WalletRepository
	cacheRepo   ports.IdempotencyRepository
	engine      ports.MatchingEngine
	gameManager *GameManager // ✨ 新增這行！把大腦接上來

}

func NewOrderService(
	orderRepo ports.OrderRepository,
	walletRepo ports.WalletRepository,
	cacheRepo ports.IdempotencyRepository,
	engine ports.MatchingEngine,
	gameManager *GameManager, // ✨ 新增這行！把大腦接上來
) *OrderService {
	return &OrderService{
		orderRepo:  orderRepo,
		walletRepo: walletRepo,
		cacheRepo:  cacheRepo,
		engine:     engine,

		gameManager: gameManager, // ✨ 存起來備用
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, order *domain.Order) error {
	fmt.Printf("🛡️ [Service層] 收到玩家 %s 的 %s 單 (價格: %.2f, 數量: %.2f)\n", order.UserID, order.Side, order.Price, order.Qty)
	if !s.gameManager.IsPlaying() {
		return errors.New("非營業時間：遊戲尚未開始或已結算結束，目前禁止下單！")
	}
	// 1. 靜態防呆檢驗
	if order.Qty <= 0 || order.Price <= 0 {
		return errors.New("無效的訂單參數")
	}
	// ✨ 新增：剛下單時，未成交數量 = 總數量；狀態 = PENDING
	order.RemainingQty = order.Qty
	order.Status = domain.Pending
	// ✨ 新增防呆：避免除以零導致保證金變成無限大 (+Inf)
	if order.Leverage <= 0 {
		order.Leverage = 1
	}

	// 2. 防重放攔截 (Redis)
	if order.IdempotencyKey != "" {
		exists, err := s.cacheRepo.ExistsOrSet(ctx, order.IdempotencyKey, 10)
		if err != nil {
			return fmt.Errorf("快取服務異常: %w", err)
		}
		if exists {
			return errors.New("重複下單")
		}
	}

	// 3. 盤前保證金凍結 (PostgreSQL)
	requiredMargin := (order.Price * order.Qty) / float64(order.Leverage)
	fmt.Printf("🛡️ [Service層] 準備凍結保證金: %.2f\n", requiredMargin) // ✨ 追蹤點

	if err := s.walletRepo.LockFunds(ctx, order.UserID, requiredMargin); err != nil {
		fmt.Printf("❌ [Service層] 資金凍結失敗: %v\n", err) // ✨ 追蹤點
		return fmt.Errorf("保證金不足: %v", err)
	}

	// 4. 訂單存檔 (PostgreSQL)
	if err := s.orderRepo.Save(ctx, order); err != nil {
		_ = s.walletRepo.ReleaseFunds(ctx, order.UserID, requiredMargin)
		return fmt.Errorf("訂單建立失敗: %w", err)
	}

	// 5. 進入撮合引擎
	fmt.Println("🛡️ [Service層] 風控通過，準備送入引擎！") // ✨ 追蹤點

	// 注意：如果你的 engine 介面沒有回傳 error，這裡直接呼叫即可
	// s.engine.PlaceOrder(order)
	if err := s.engine.PlaceOrder(order); err != nil {
		return fmt.Errorf("撮合引擎異常: %w", err)
	}

	return nil
}

func (s *OrderService) CancelOrder(ctx context.Context, orderID string, userID string) error {
	fmt.Printf("🛡️ [Service層] 收到玩家 %s 請求撤銷訂單 %s\n", userID, orderID)

	// 1. 從資料庫撈出這筆訂單
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("查詢訂單失敗: %v", err)
	}
	if order == nil {
		return errors.New("查無此訂單")
	}
	// ✨ 就是這段！請確認它有乖乖待在這裡並存檔！
	if order.RemainingQty <= 0 {
		return errors.New("撤單失敗：訂單已完全交割，無法撤銷")
	}
	// 防駭客：只能撤銷自己的單
	if order.UserID != userID {
		return errors.New("越權操作：無法撤銷他人的訂單")
	}

	// ✨ 終極防線 1：檢查訂單狀態！如果已經完全成交或已撤銷，直接擋下！
	if order.Status == domain.Filled {
		return errors.New("撤單失敗：訂單已完全成交")
	}
	if order.Status == domain.Canceled {
		return errors.New("撤單失敗：訂單已在撤銷狀態，請勿重複點擊")
	}

	// 2. 從引擎中抽單
	if err := s.engine.CancelOrder(orderID); err != nil {
		return fmt.Errorf("引擎撤單失敗 (可能就在這毫秒間成交了): %w", err)
	}

	// ✨ 終極防線 2：使用「RemainingQty (剩餘數量)」來計算退款，而不是原始 Qty！
	if order.Leverage <= 0 {
		order.Leverage = 1
	}
	// 這裡的退款精準度決定了交易所會不會破產
	refundMargin := (order.Price * order.RemainingQty) / float64(order.Leverage)

	// 3. 退還保證金
	if err := s.walletRepo.ReleaseFunds(ctx, userID, refundMargin); err != nil {
		return fmt.Errorf("解凍保證金失敗: %v", err)
	}

	// 4. 更新狀態為 CANCELLED
	if err := s.orderRepo.UpdateStatus(ctx, orderID, domain.Canceled); err != nil {
		return fmt.Errorf("更新訂單狀態失敗: %w", err)
	}

	fmt.Printf("✅ [Service層] 訂單 %s 成功撤銷，退還保證金 $%.2f (剩餘未成交: %.2f張)\n", orderID, refundMargin, order.RemainingQty)
	return nil
}
