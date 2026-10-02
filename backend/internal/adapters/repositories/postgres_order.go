package repositories

import (
	"context"
	"database/sql"
	"ntpufc-exchange/backend/internal/core/domain"
)

type PostgresOrderRepo struct {
	db *sql.DB
}

func NewPostgresOrderRepo(db *sql.DB) (*PostgresOrderRepo, error) {
	repo := &PostgresOrderRepo{db: db}
	if err := repo.initSchema(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *PostgresOrderRepo) initSchema() error {
	// ✨ 開發階段大絕招：每次啟動先把舊的髒資料表砍了，強制套用最新結構！
	// (注意：未來正式上線辦茶會時，記得把這行註解掉，不然一重開機玩家訂單就全沒了XD)
	r.db.Exec("DROP TABLE IF EXISTS orders;")
	// ✨ 升級：資料表新增了 remaining_qty 欄位
	query := `
	CREATE TABLE IF NOT EXISTS orders (
		id VARCHAR(50) PRIMARY KEY,
		user_id VARCHAR(50) NOT NULL,
		symbol VARCHAR(20) NOT NULL,
		side VARCHAR(10) NOT NULL,
		price NUMERIC(15, 2) NOT NULL,
		qty NUMERIC(15, 2) NOT NULL,
		remaining_qty NUMERIC(15, 2) NOT NULL, 
		leverage INT NOT NULL DEFAULT 1,
		status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
	);
	`
	// ⚠️ 備註：如果你之前已經啟動過伺服器，資料庫裡會有舊的 orders 表。
	// 若遇到「欄位不存在」的錯誤，你可以先在 PostgreSQL 裡手動下 DROP TABLE orders; 讓它重新建立。
	_, err := r.db.Exec(query)
	return err
}

// Save 將新訂單存入資料庫
func (r *PostgresOrderRepo) Save(ctx context.Context, order *domain.Order) error {
	// ✨ 升級：存入 remaining_qty 與明確的 Status
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO orders (id, user_id, symbol, side, price, qty, remaining_qty, leverage, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, order.ID, order.UserID, order.Symbol, string(order.Side), order.Price, order.Qty, order.RemainingQty, order.Leverage, string(order.Status))
	return err
}

// FindByID 從資料庫撈出訂單
func (r *PostgresOrderRepo) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	order := &domain.Order{}
	var sideStr, statusStr string

	// ✨ 升級：把 remaining_qty 跟 status 一併撈出來
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, symbol, side, price, qty, remaining_qty, leverage, status
		FROM orders WHERE id = $1
	`, id).Scan(&order.ID, &order.UserID, &order.Symbol, &sideStr, &order.Price, &order.Qty, &order.RemainingQty, &order.Leverage, &statusStr)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	order.Side = domain.OrderSide(sideStr)
	order.Status = domain.OrderStatus(statusStr)
	return order, nil
}

// UpdateStatus 更新訂單狀態
func (r *PostgresOrderRepo) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus) error {
	_, err := r.db.ExecContext(ctx, "UPDATE orders SET status = $1 WHERE id = $2", string(status), id)
	return err
}

// ✨ 新增防護功能：當引擎撮合成功時，用這個方法扣減未成交數量，並自動判定是否完全成交
func (r *PostgresOrderRepo) UpdateOrderAfterMatch(ctx context.Context, tx *sql.Tx, orderID string, matchedQty float64) error {
	// 這裡使用 tx (Transaction) 是為了跟 Wallet 的扣款包在同一個交易裡，保證原子性
	_, err := tx.ExecContext(ctx, `
		UPDATE orders 
		SET remaining_qty = remaining_qty - $1,
			status = CASE 
				WHEN remaining_qty - $1 <= 0 THEN 'FILLED'
				ELSE 'PARTIAL_FILLED'
			END
		WHERE id = $2
	`, matchedQty, orderID)
	return err
}
