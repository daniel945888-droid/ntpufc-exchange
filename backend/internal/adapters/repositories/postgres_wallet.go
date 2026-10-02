package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time" // ✨ 記得要有這個，才能產生 matchID

	"ntpufc-exchange/backend/internal/core/domain"
)

type PostgresWalletRepo struct {
	db *sql.DB
}

func NewPostgresWalletRepo(db *sql.DB) (*PostgresWalletRepo, error) {
	repo := &PostgresWalletRepo{db: db}
	if err := repo.initSchema(); err != nil {
		return nil, err
	}
	return repo, nil
}

// ✨ 升級 1：加入 positions (資產庫存) 資料表
func (r *PostgresWalletRepo) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS wallets (
		user_id VARCHAR(50) PRIMARY KEY,
		available_balance NUMERIC(15, 2) NOT NULL DEFAULT 10000.00,
		locked_balance NUMERIC(15, 2) NOT NULL DEFAULT 0.00
	);

	CREATE TABLE IF NOT EXISTS positions (
		user_id VARCHAR(50),
		symbol VARCHAR(20),
		qty NUMERIC(15, 2) NOT NULL DEFAULT 0.00,
		PRIMARY KEY (user_id, symbol)
	);
	`
	_, err := r.db.Exec(query)
	return err
}

func (r *PostgresWalletRepo) LockFunds(ctx context.Context, userID string, amount float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallets (user_id, available_balance, locked_balance)
		VALUES ($1, 10000.00, 0.00)
		ON CONFLICT (user_id) DO NOTHING;
	`, userID)
	if err != nil {
		return fmt.Errorf("初始化玩家錢包失敗: %w", err)
	}

	var available float64
	err = tx.QueryRowContext(ctx, `
		SELECT available_balance 
		FROM wallets 
		WHERE user_id = $1 
		FOR UPDATE
	`, userID).Scan(&available)
	if err != nil {
		return err
	}

	if available < amount {
		return errors.New("可用保證金不足")
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE wallets 
		SET available_balance = available_balance - $2, 
		    locked_balance = locked_balance + $2 
		WHERE user_id = $1
	`, userID, amount)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PostgresWalletRepo) ReleaseFunds(ctx context.Context, userID string, amount float64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE wallets 
		SET available_balance = available_balance + $2, 
		    locked_balance = locked_balance - $2 
		WHERE user_id = $1
	`, userID, amount)
	return err
}

func (r *PostgresWalletRepo) GetBalance(ctx context.Context, userID string) (float64, float64, error) {
	var available, locked float64
	err := r.db.QueryRowContext(ctx, "SELECT available_balance, locked_balance FROM wallets WHERE user_id = $1", userID).Scan(&available, &locked)
	return available, locked, err
}

func (r *PostgresWalletRepo) SettleTrade(ctx context.Context, event domain.TradeEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	totalAmount := event.Price * event.Qty

	// ==========================================
	// 💰 1. 現金結算 (Cash Settlement)
	// ==========================================

	// 1.1 買方：扣除已凍結的保證金 (可用餘額在下單時已扣除，交割時只需把凍結的錢「真正銷毀」)
	// 注意：下單時 LockFunds 是扣 available，加 locked。所以這裡只要扣 locked 即可。
	_, err = tx.ExecContext(ctx, `
		UPDATE wallets 
		SET locked_balance = locked_balance - $1 
		WHERE user_id = $2
	`, totalAmount, event.BuyerID)
	if err != nil {
		return fmt.Errorf("買方解除凍結保證金失敗: %w", err)
	}

	// 1.2 賣方：獲得現金 (總額)
	// 這裡假設賣單不預扣現金保證金，交割成功直接把錢加進 available_balance
	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallets (user_id, available_balance, locked_balance) 
		VALUES ($1, 10000.00 + $2, 0)
		ON CONFLICT (user_id) DO UPDATE 
		SET available_balance = wallets.available_balance + $2
	`, event.SellerID, totalAmount)
	if err != nil {
		return fmt.Errorf("賣方現金入帳失敗: %w", err)
	}

	// ==========================================
	// 📦 2. 股票庫存結算 (Position Settlement)
	// ==========================================

	// 2.1 買方：獲得股票
	_, err = tx.ExecContext(ctx, `
		INSERT INTO positions (user_id, symbol, qty) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, symbol) DO UPDATE SET qty = positions.qty + $3
	`, event.BuyerID, event.Symbol, event.Qty)
	if err != nil {
		return fmt.Errorf("買方入帳股票失敗: %w", err)
	}

	// 2.2 賣方：扣除股票 (容許放空產生負數)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO positions (user_id, symbol, qty) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, symbol) DO UPDATE SET qty = positions.qty - $3
	`, event.SellerID, event.Symbol, event.Qty) // ✨ 注意這裡我改成了 $3 (正數)，並在 SQL 裡用 - $3 扣除
	if err != nil {
		return fmt.Errorf("賣方扣除股票失敗: %w", err)
	}

	// ==========================================
	// 📝 3. 訂單狀態與明細紀錄 (Order Status & Logs)
	// ==========================================

	// 3.1 買方訂單狀態更新
	_, err = tx.ExecContext(ctx, `
		UPDATE orders 
		SET remaining_qty = remaining_qty - $1,
			status = CASE 
				WHEN remaining_qty - $1 <= 0 THEN 'FILLED'
				ELSE 'PARTIAL_FILLED'
			END
		WHERE id = $2
	`, event.Qty, event.BuyerOrderID)
	if err != nil {
		return fmt.Errorf("買方訂單狀態更新失敗: %w", err)
	}

	// 3.2 賣方訂單狀態更新
	_, err = tx.ExecContext(ctx, `
		UPDATE orders 
		SET remaining_qty = remaining_qty - $1,
			status = CASE 
				WHEN remaining_qty - $1 <= 0 THEN 'FILLED'
				ELSE 'PARTIAL_FILLED'
			END
		WHERE id = $2
	`, event.Qty, event.SellerOrderID)
	if err != nil {
		return fmt.Errorf("賣方訂單狀態更新失敗: %w", err)
	}

	// 3.3 將歷史明細寫入 match_logs
	matchID := fmt.Sprintf("MATCH-%d", time.Now().UnixNano())
	_, err = tx.ExecContext(ctx, `
		INSERT INTO match_logs (id, buyer_order_id, seller_order_id, symbol, execute_price, execute_qty)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, matchID, event.BuyerOrderID, event.SellerOrderID, event.Symbol, event.Price, event.Qty)
	if err != nil {
		return fmt.Errorf("明細寫入失敗: %w", err)
	}

	fmt.Printf("⚖️ [清算交割] 買方 %s 與 賣方 %s 成功交割 %s %.2f股 (總價 $%.2f)\n", event.BuyerID, event.SellerID, event.Symbol, event.Qty, totalAmount)

	return tx.Commit()
}
func (r *PostgresWalletRepo) GetPosition(ctx context.Context, userID string, symbol string) (float64, error) {
	var qty float64
	err := r.db.QueryRowContext(ctx, "SELECT qty FROM positions WHERE user_id = $1 AND symbol = $2", userID, symbol).Scan(&qty)

	// 如果找不到紀錄 (還沒買過)，就回傳 0
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0.0, nil
		}
		return 0.0, err
	}
	return qty, nil
}
