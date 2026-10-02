import { create } from 'zustand';

export interface DepthLevel {
  price: number;
  qty: number;
}

// 💡 新增：未成交訂單的資料格式
export interface OpenOrder {
  id: string;
  side: 'buy' | 'sell';
  price: number;
  qty: number;
}

interface TradeStore {
  isConnected: boolean;
  balance: number;
  position: number;
  lastPrice: number; // ✨ 新增這行：記錄最新市價
  bids: DepthLevel[];
  asks: DepthLevel[];
  logs: string[];
  openOrders: OpenOrder[]; // 💡 新增：存放玩家自己的掛單
  gameStatus: 'WAITING' | 'PLAYING' | 'ENDED'; // ✨ 新增這行
  gameEndTime: number | null; // ✨ 新增：存放絕對時間戳

  
  connect: (userId: string) => void;
  placeOrder: (symbol: string, price: number, qty: number, side: 'buy' | 'sell') => void;
  cancelOrder: (orderId: string) => void; // 💡 新增：發送撤單指令
  addLog: (msg: string) => void;
}

let wsInstance: WebSocket | null = null;

export const useTradeStore = create<TradeStore>((set, get) => ({
  isConnected: false,
  balance: 10000,
  position: 0,
  lastPrice: 100, // ✨ 新增預設市價
  bids: [],
  asks: [],
  logs: [],
  openOrders: [],
  gameStatus: 'WAITING', // ✨ 新增預設值
  gameEndTime: null, // ✨ 新增預設值
  addLog: (msg: string) => set((state) => ({ logs: [msg, ...state.logs].slice(0, 10) })),

  connect: (userId: string) => {
    if (wsInstance && wsInstance.readyState === WebSocket.OPEN) return;

    // ✨ 核心修復：動態判定 WebSocket 網址，絕對不能寫死 localhost！
    const currentHostname = window.location.hostname;
    const currentHost = window.location.host; // 包含 port
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    
    let wsUrl = '';
    // 如果是電腦本機開發 (localhost)，直接連向 Go 的 8080
    if (currentHostname === 'localhost' || currentHostname === '127.0.0.1') {
      wsUrl = `ws://localhost:8080/ws?user_id=${userId}`;
    } else {
      // 如果是手機外網 (Cloudflare)，連向當前網域，讓 Vite Proxy 幫忙轉發給 Go
      wsUrl = `${protocol}//${currentHost}/ws?user_id=${userId}`;
    }

    // 💡 印在畫面上，讓我們百分之百確認手機連對地方了
    get().addLog(`🔍 嘗試連線至: ${wsUrl}`);

    wsInstance = new WebSocket(wsUrl);

    wsInstance.onopen = (event) => {
      set({ isConnected: true });
      get().addLog('✅ 系統連線成功');
      
      // ✨ 終極防護：抓取真正觸發 open 事件的 websocket 實例，確認它已開啟才發送
      const ws = event.target as WebSocket;
      if (ws.readyState === WebSocket.OPEN) {
         ws.send(JSON.stringify({ action: 'get_status' }));
      }
    };

    wsInstance.onmessage = (event) => {
        try {
          console.log("📥 [WS 原始訊息]:", event.data);
          const data = JSON.parse(event.data);
          console.log("🔍 [解析後]:", data);

          // 🛡️ 終極防護：如果封包是空的，直接略過
          if (!data) return;

          // ✨ 1. 攔截遊戲狀態與設定倒數時間 (已修復合併！)
          if (data.type === 'game_state_sync') {
             const newState = data.message?.state;
             const serverEndTime = data.message?.end_time; // ✨ 從後端抓取真實結束時間
             if (newState) {  
              set({ 
                 gameStatus: newState,
                 // ✨ 只要後端有傳時間，就絕對服從後端！否則才自己算
                 gameEndTime: serverEndTime || (newState === 'PLAYING' ? Date.now() + 600000 : null)
               });
             }
             
             // 🛡️ 已修正為 data.message?.message，絕不崩潰
             if (data.message?.message) {
               get().addLog(`📢 系統廣播: ${data.message.message}`);
             }
             return;
          }

          // ❌ (原本第二個舊的 data.event === 'game_state_sync' 已經幫你刪除)

          if (data.error) {
            get().addLog(`❌ 錯誤: ${data.error}`);
            return;
          }

          // 💡 攔截 ORDER_ACK
          if (data.type === 'ORDER_ACK') {
            // ✨ 終極防呆：直接從 data 拿值，不要再被 message 字串騙了！
            const actualId = data.order_id || data.id || data.orderId;

            set((state) => ({
              openOrders: [{ 
                id: actualId, 
                side: data.side, 
                price: data.price, 
                qty: data.qty 
              }, ...state.openOrders]
            }));
            
            get().addLog(`📤 ${data.message || '委託成功'} (ID: ${actualId})`);
            return;
          }
          // 💡 攔截交割成功通知 (TRADE_SUCCESS)
          if (data.type === 'TRADE_SUCCESS') {
            set((state) => ({
              // 直接拿後端算好的最新數據，如果沒傳則保持原樣
              balance: data.new_balance !== undefined ? data.new_balance : state.balance,
              position: data.new_position !== undefined ? data.new_position : state.position,
              lastPrice: data.price !== undefined ? data.price : state.lastPrice,
            }));
            
            // 把後端傳來的精美文字印在日誌上
            get().addLog(`🤝 ${data.message}`);
            return;
          }
          // 💡 攔截 CANCEL_ACK
          if (data.type === 'CANCEL_ACK') {
            // ✨ 一樣直接從 data 拿 ID，避開 message 字串陷阱！
            const actualId = data.order_id || data.id || data.orderId;
            
            set((state) => ({
              openOrders: state.openOrders.filter(o => o.id !== actualId)
            }));
            
            get().addLog(`🗑️ ${data.message || '撤單成功'} (ID: ${actualId})`);
            return;
          }

          // 💡 攔截 MARKET_DEPTH
          if (data.type === 'MARKET_DEPTH') {
            // 🛡️ 加上 ?. 防呆，沒資料就給空陣列
            set({ 
              bids: data.message?.bids || [], 
              asks: data.message?.asks || [] 
            });
            return;
          }

          // 💡 攔截 TRADE_SUCCESS
          if (data.type === 'TRADE_SUCCESS') {
              const trade = data.message;
              if (!trade) return; // 🛡️ 防呆

              get().addLog(`💰 ${trade.message}`); 

              set((state) => {
                  const cost = trade.price * trade.qty;
                  
                  // 1. 計算新的餘額與部位
                  const newBalance = trade.side === 'buy' ? state.balance - cost : state.balance + cost;
                  const newPosition = trade.side === 'buy' ? state.position + trade.qty : state.position - trade.qty;

                  // 2. 從 UI 掛單列表中扣除成交數量
                  const updatedOrders = state.openOrders
                      .map(order => {
                          if (order.id === trade.order_id) {
                              return { ...order, qty: order.qty - trade.qty };
                          }
                          return order;
                      })
                      .filter(order => order.qty > 0);

                  // 3. 一次性更新所有狀態
                  return { 
                      balance: newBalance, 
                      position: newPosition,
                      openOrders: updatedOrders 
                  };
              });
              return;
          }

          // 💡 攔截其他未分類的純文字訊息
          if (data.message && typeof data.message === 'string') {
             get().addLog(`📩 ${data.message}`);
          }

        } catch (err) {
          console.error("解析 WebSocket 訊息失敗:", err, "原始封包:", event.data);
        }
      };
  },

  placeOrder: (symbol, price, qty, side) => {
    if (!wsInstance) return;
    wsInstance.send(JSON.stringify({ action: 'place_order', symbol, price, qty, side, leverage: 1 }));
  },

  cancelOrder: (orderId) => {
    if (!wsInstance) return;
    wsInstance.send(JSON.stringify({ action: 'cancel_order', order_id: orderId }));
  }
}));