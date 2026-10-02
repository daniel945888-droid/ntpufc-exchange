import { useEffect, useState } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate, Link } from 'react-router-dom';
import { useTradeStore } from './store/useTradeStore';
import { TradingChart } from './components/TradingChart';
import { GameTimer } from './components/GameTimer';
import { LoginWall } from './pages/LoginWall';
import './App.css';

// ==========================================
// 1. 玩家端交易終端 (也就是你原本的 App 內容)
// ==========================================
function TradingTerminal() {
// ✨ 把 gameStatus 也解構出來
  // ✨ 把 lastPrice 解構出來
  const { isConnected, gameStatus, balance, position, bids, asks, logs, openOrders, connect, placeOrder, cancelOrder, lastPrice } = useTradeStore();
  const [inputPrice, setInputPrice] = useState<number | string>(100);
  const [inputQty, setInputQty] = useState<number | string>(1);
  const isTradingLocked = gameStatus !== 'PLAYING';

  // ✨ 動態計算總資產與損益
  const totalAssets = balance + (position * lastPrice);
  const initialCapital = 10000; // 開局本金
  const pnl = totalAssets - initialCapital; // 未實現損益
  
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    // 從參數拿出 user_id。萬一網址沒帶這個參數，就給他一個 Guest_隨機碼
    const currentUser = params.get('user_id') || `Guest_${Math.floor(Math.random() * 1000)}`;
    connect(currentUser);
  }, [connect]);

  const handleRealOrder = (side: 'buy' | 'sell') => {
    
    const price = Number(inputPrice);
    const qty = Number(inputQty);
    if (isTradingLocked) {
      alert("⚠️ 非營業時間：目前無法下單！");
      return;
    }
    if (!price || !qty || price <= 0 || qty <= 0) {
      alert("⚠️ 請輸入有效的價格與數量大於 0！");
      return;
    }
    placeOrder('AI_STOCK', price, qty, side);
  };

  return (
    <div style={{ padding: '20px', fontFamily: 'sans-serif', backgroundColor: '#121212', color: '#ffffff', minHeight: '100vh' }}>
      {/* 導航捷徑：方便測試時快速切回主持人端 */}
      <div style={{ marginBottom: '10px', display: 'flex', justifyContent: 'flex-end' }}>
        <Link to="/admin" style={{ color: '#818cf8', textDecoration: 'none', fontSize: '0.9rem' }}>👑 切換至主持人戰情室</Link>
      </div>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '15px' }}>
        <h1 style={{ margin: 0 }}>📈 迎新茶會 - 交易大戰室</h1>
        {/* ✨ 在標題列中間插入計時器 */}
        <GameTimer />
        <span style={{ color: isConnected ? '#00ff00' : '#ff3333', fontWeight: 'bold', backgroundColor: '#1e1e1e', padding: '8px 15px', borderRadius: '20px' }}>
          {isConnected ? '🟢 已連線至撮合引擎' : '🔴 離線中'}
        </span>
      </div>
      
      <div style={{ marginBottom: '20px' }}>
        <TradingChart />
      </div>
      
      <div style={{ display: 'flex', gap: '20px', alignItems: 'flex-start' }}>
        
        {/* 左側：資產、操作與個人委託區 */}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '20px' }}>
          
          <div style={{ padding: '15px', border: '1px solid #333', borderRadius: '8px', backgroundColor: '#1e1e1e' }}>
            {/* 替換原本的 💰 帳戶資產 區塊 */}
          <div style={{ padding: '15px', border: '1px solid #333', borderRadius: '8px', backgroundColor: '#1e1e1e' }}>
            <h2 style={{ margin: '0 0 15px 0' }}>💰 帳戶資產</h2>
            
            <h3 style={{ color: '#fff', margin: '0 0 10px 0', fontSize: '1.5rem' }}>
              總資產：${totalAssets.toLocaleString()}
              <span style={{ fontSize: '1.2rem', marginLeft: '15px', color: pnl >= 0 ? '#00ff00' : '#ff4444' }}>
                {pnl >= 0 ? '▲' : '▼'} ${Math.abs(pnl).toLocaleString()}
              </span>
            </h3>
            
            <hr style={{ borderColor: '#333', margin: '15px 0' }} />
            
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px', fontSize: '1.1rem' }}>
              <span style={{ color: '#00ffff' }}>可用現金：</span>
              <span style={{ color: '#00ffff', fontWeight: 'bold' }}>${balance.toLocaleString()}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '8px', fontSize: '1.1rem' }}>
              <span style={{ color: '#ff9900' }}>庫存價值：</span>
              <span style={{ color: '#ff9900', fontWeight: 'bold' }}>${(position * lastPrice).toLocaleString()}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.9rem', color: '#aaa', marginTop: '10px' }}>
              <span>目前持倉：{position} 張</span>
              <span>最新市價：${lastPrice}</span>
            </div>
          </div>
          {/* ======================= */}
            
            <h2>⚡ 交易委託</h2>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '15px' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <label style={{ color: '#aaa', fontWeight: 'bold' }}>委託價格 ($)</label>
                <input type="number" value={inputPrice} onChange={(e) => setInputPrice(e.target.value)} style={{ padding: '10px', width: '60%', backgroundColor: '#000', color: '#fff', border: '1px solid #555', borderRadius: '5px', fontSize: '16px', textAlign: 'right' }} />
              </div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <label style={{ color: '#aaa', fontWeight: 'bold' }}>委託數量 (張)</label>
                <input type="number" value={inputQty} onChange={(e) => setInputQty(e.target.value)} style={{ padding: '10px', width: '60%', backgroundColor: '#000', color: '#fff', border: '1px solid #555', borderRadius: '5px', fontSize: '16px', textAlign: 'right' }} />
              </div>
              <div style={{ display: 'flex', gap: '10px', marginTop: '10px' }}>
              <button 
          onClick={() => handleRealOrder('buy')} 
          disabled={isTradingLocked}
          style={{ flex: 1, padding: '15px', fontSize: '18px', backgroundColor: '#00cc00', color: '#000', border: 'none', borderRadius: '5px', cursor: isTradingLocked ? 'not-allowed' : 'pointer', fontWeight: 'bold', opacity: isTradingLocked ? 0.3 : 1 }}
        >買入 (Buy)</button>
        
        <button 
          onClick={() => handleRealOrder('sell')} 
          disabled={isTradingLocked}
          style={{ flex: 1, padding: '15px', fontSize: '18px', backgroundColor: '#cc0000', color: '#fff', border: 'none', borderRadius: '5px', cursor: isTradingLocked ? 'not-allowed' : 'pointer', fontWeight: 'bold', opacity: isTradingLocked ? 0.3 : 1 }}
        >賣出 (Sell)</button>
              </div>
            </div>
          </div>

          <div style={{ padding: '15px', border: '1px solid #333', borderRadius: '8px', backgroundColor: '#1e1e1e', flex: 1 }}>
            <h2>📋 當前掛單 ({openOrders.length})</h2>
            {openOrders.length === 0 ? (
              <p style={{ color: '#777', textAlign: 'center', marginTop: '30px' }}>目前沒有未成交的委託單</p>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', maxHeight: '250px', overflowY: 'auto' }}>
                {openOrders.map(order => (
                  <div key={order.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '10px', backgroundColor: '#000', borderLeft: `5px solid ${order.side === 'buy' ? '#00cc00' : '#cc0000'}`, borderRadius: '4px' }}>
                    <div>
                      <span style={{ color: order.side === 'buy' ? '#00cc00' : '#ff4444', fontWeight: 'bold', marginRight: '10px' }}>
                        {order.side === 'buy' ? '買' : '賣'}
                      </span>
                      <span style={{ marginRight: '10px' }}>${order.price}</span>
                      <span style={{ color: '#aaa' }}>{order.qty} 張</span>
                    </div>
                    <button onClick={() => cancelOrder(order.id)} style={{ backgroundColor: 'transparent', color: '#ff4444', border: '1px solid #ff4444', padding: '5px 10px', borderRadius: '4px', cursor: 'pointer', fontWeight: 'bold' }}>✖ 撤單</button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* 中間：掛單簿 */}
        <div style={{ flex: 1, padding: '15px', border: '1px solid #333', borderRadius: '8px', backgroundColor: '#181818', minHeight: '380px' }}>
          <h2 style={{ textAlign: 'center', margin: '0 0 15px 0' }}>📊 市場深度</h2>
          <div style={{ display: 'flex', justifyContent: 'space-between', color: '#888', borderBottom: '1px solid #333', paddingBottom: '5px', marginBottom: '10px' }}>
            <span style={{ width: '33%' }}>買/賣</span><span style={{ width: '33%', textAlign: 'center' }}>價格</span><span style={{ width: '33%', textAlign: 'right' }}>數量</span>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column-reverse', minHeight: '120px', justifyContent: 'flex-start' }}>
            {asks.slice(0, 5).map((ask, idx) => (
              <div key={`ask-${idx}`} style={{ display: 'flex', justifyContent: 'space-between', color: '#ff4444', padding: '4px 0', backgroundColor: 'rgba(255, 68, 68, 0.05)' }}>
                <span style={{ width: '33%' }}>賣</span><span style={{ width: '33%', textAlign: 'center', fontWeight: 'bold' }}>{ask.price}</span><span style={{ width: '33%', textAlign: 'right' }}>{ask.qty}</span>
              </div>
            ))}
          </div>
          <div style={{ textAlign: 'center', margin: '10px 0', color: '#666', borderTop: '1px dashed #444', borderBottom: '1px dashed #444', padding: '5px 0' }}>買賣價差區</div>
          <div style={{ minHeight: '120px' }}>
            {bids.slice(0, 5).map((bid, idx) => (
              <div key={`bid-${idx}`} style={{ display: 'flex', justifyContent: 'space-between', color: '#00ff00', padding: '4px 0', backgroundColor: 'rgba(0, 255, 0, 0.05)' }}>
                <span style={{ width: '33%' }}>買</span><span style={{ width: '33%', textAlign: 'center', fontWeight: 'bold' }}>{bid.price}</span><span style={{ width: '33%', textAlign: 'right' }}>{bid.qty}</span>
              </div>
            ))}
          </div>
        </div>

        {/* 右側：日誌區 */}
        <div style={{ flex: 1, padding: '15px', border: '1px solid #333', borderRadius: '8px', backgroundColor: '#000000', fontFamily: 'monospace', minHeight: '380px', overflowY: 'auto' }}>
          <h2 style={{ margin: '0 0 15px 0' }}>📜 終端機日誌</h2>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', fontSize: '13px' }}>
            {logs.map((log, idx) => (
              <span key={idx} style={{ color: log.includes('❌') || log.includes('⚠️') ? '#ff4444' : log.includes('💰') ? '#00ff00' : '#cccccc', lineHeight: '1.4' }}>{log}</span>
            ))}
          </div>
        </div>

      </div>
    </div>
  );
}

// ==========================================
// 2. 主持人大螢幕戰情室 (Admin Dashboard)
// ==========================================
function AdminDashboard() {
  // ✨ 1. 拔掉 useState，改從 useTradeStore 拿全域狀態與連線函數
  const { gameStatus, connect } = useTradeStore();
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');

  // ✨ 2. 讓主持人頁面一載入，就連上 WebSocket (化身為 'admin_host' 監聽廣播)
  useEffect(() => {
    connect('admin_host');
  }, [connect]);
  const handleStartGame = async () => {
    try {
      setLoading(true);
      const response = await fetch('/api/admin/start', { method: 'POST' });
      
      const text = await response.text();
      if (response.ok) {
        // ✨ 3. 這裡不再需要 setGameStatus('PLAYING')，
        // 因為後端收到請求後會發送 WebSocket 廣播，我們的 Store 會自動把全場變成 PLAYING！
        setMessage("指令已送出，等待系統廣播開局...");
      } else {
        setMessage(`啟動失敗: ${text}`);
      }
    } catch (err) {
      setMessage('無法連線至後端伺服器，請確認 Go 服務是否啟動！');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: '40px', backgroundColor: '#121212', color: '#fff', minHeight: '100vh', fontFamily: 'sans-serif' }}>
      <header style={{ textAlign: 'center', marginBottom: '40px' }}>
        <h1 style={{ fontSize: '2.5rem', color: '#4f46e5' }}>NTPUFC 高頻交易所 - 主持人戰情室</h1>
        {/* ✨ 主持人畫面上方也放一個超級大時鐘 */}
        <div style={{ transform: 'scale(1.5)', marginTop: '20px' }}>
          <GameTimer />
        </div>
        <p style={{ color: '#9ca3af' }}>迎新茶會現場大螢幕控場中心</p>
        <div style={{ marginTop: '15px' }}>
          <Link to="/trade" style={{ color: '#34d399', textDecoration: 'none', fontSize: '1rem', border: '1px solid #34d399', padding: '6px 12px', borderRadius: '6px' }}>
            🔗 前往玩家交易端 (/trade)
          </Link>
        </div>
      </header>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '30px', maxWidth: '1200px', margin: '0 auto' }}>
        
        {/* 左側：控制面板 */}
        <div style={{ backgroundColor: '#1e1e2f', padding: '30px', borderRadius: '12px', border: '1px solid #374151' }}>
          <h2>🎮 遊戲控制中樞</h2>
          <div style={{ margin: '20px 0' }}>
            <p>目前遊戲狀態：<span style={{ fontWeight: 'bold', color: gameStatus === 'PLAYING' ? '#10b981' : '#f59e0b' }}>{gameStatus}</span></p>
          </div>

          <button 
            onClick={handleStartGame}
            disabled={gameStatus === 'PLAYING' || loading}
            style={{
              width: '100%',
              padding: '16px',
              backgroundColor: gameStatus === 'PLAYING' ? '#4b5563' : '#4f46e5',
              color: '#fff',
              fontSize: '1.2rem',
              fontWeight: 'bold',
              border: 'none',
              borderRadius: '8px',
              cursor: gameStatus === 'PLAYING' ? 'not-allowed' : 'pointer',
            }}
          >
            {loading ? '啟動中...' : gameStatus === 'PLAYING' ? '遊戲進行中 ⚔️' : '🚀 啟動 10 分鐘交易回合'}
          </button>

          {message && (
            <div style={{ marginTop: '20px', padding: '12px', backgroundColor: '#374151', borderRadius: '6px', color: '#34d399' }}>
              {message}
            </div>
          )}
        </div>

        {/* 右側：QR Code 與玩家引導區 */}
        <div style={{ backgroundColor: '#1e1e2f', padding: '30px', borderRadius: '12px', border: '1px solid #374151', textAlign: 'center' }}>
          <h2>📱 玩家連線專區</h2>
          <p style={{ color: '#9ca3af', marginBottom: '20px' }}>請台下學員使用手機相機掃描下方網址</p>
          
          <div style={{ width: '200px', height: '200px', backgroundColor: '#fff', margin: '0 auto', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#000', borderRadius: '8px', fontWeight: 'bold' }}>
            QR Code 區域
          </div>
          <p style={{ marginTop: '15px', fontSize: '0.9rem', color: '#9ca3af' }}>
            玩家端網址：<code>http://&lt;你的IP&gt;:5173/trade</code>
          </p>
        </div>

      </div>
    </div>
  );
}

// ==========================================
// 3. 根目錄路由總管 (App)
// ==========================================
export function App() {
  return (
    <Router>
      <Routes>
        {/* ✨ 修改 1：預設首頁改成玩家的「登入牆」 */}
        <Route path="/" element={<LoginWall />} />
        
        {/* 主持人大螢幕介面 */}
        <Route path="/admin" element={<AdminDashboard />} />
        
        {/* 玩家手機端介面 */}
        <Route path="/trade" element={<TradingTerminal />} />
      </Routes>
    </Router>
  );
}

export default App;