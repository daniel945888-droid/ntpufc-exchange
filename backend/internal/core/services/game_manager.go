package services

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// 定義遊戲狀態
type GameState string

const (
	StateWaiting GameState = "WAITING" // 等待主持人按開始
	StatePlaying GameState = "PLAYING" // 10 分鐘交易時間
	StateEnded   GameState = "ENDED"   // 時間到，強制結算
)

// GameManager 掌控全域的遊戲回合狀態
type GameManager struct {
	mu            sync.RWMutex
	state         GameState
	roundDuration time.Duration
	endTime       time.Time

	// 廣播回呼函數：用來通知 ws_pool 把狀態推播給所有玩家
	onStateChange func(state GameState, message string)
}

// NewGameManager 建立新的遊戲大腦 (預設 10 分鐘)
func NewGameManager(duration time.Duration, broadcastCb func(GameState, string)) *GameManager {
	return &GameManager{
		state:         StateWaiting,
		roundDuration: duration,
		onStateChange: broadcastCb,
	}
}

// StartRound 主持人呼叫：開始遊戲並啟動倒數計時
func (gm *GameManager) StartRound() error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if gm.state == StatePlaying {
		return errors.New("遊戲已經在進行中，請勿重複啟動")
	}

	gm.state = StatePlaying
	gm.endTime = time.Now().Add(gm.roundDuration)

	fmt.Printf("🎮 [GameManager] 遊戲開始！預計結束時間: %v\n", gm.endTime.Format("15:04:05"))

	// 透過回呼函數廣播給全伺服器玩家
	if gm.onStateChange != nil {
		gm.onStateChange(gm.state, "遊戲開始！請開始交易。")
	}

	// 啟動 Goroutine 在背景偷偷倒數 10 分鐘
	go gm.countdown()

	return nil
}

// countdown 背景倒數計時器
func (gm *GameManager) countdown() {
	gm.mu.RLock()
	target := gm.endTime
	gm.mu.RUnlock()

	duration := time.Until(target)
	if duration > 0 {
		time.Sleep(duration) // 讓這個 Goroutine 睡 10 分鐘
	}

	// 睡醒了，代表時間到，強制結束回合！
	gm.EndRound()
}

// EndRound 遊戲結束：鎖死交易並廣播
func (gm *GameManager) EndRound() {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if gm.state != StatePlaying {
		return
	}

	gm.state = StateEnded
	fmt.Println("🛑 [GameManager] 時間到！本輪交易結束，準備結算...")

	if gm.onStateChange != nil {
		gm.onStateChange(gm.state, "時間到！本輪交易已結束，請看大螢幕結算成績。")
	}
}

// IsPlaying 🛡️ 最重要的防護鎖：提供給 OrderService 檢查現在能不能下單
func (gm *GameManager) IsPlaying() bool {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.state == StatePlaying
}
