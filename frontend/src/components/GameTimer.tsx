import { useState, useEffect } from 'react';
import { useTradeStore } from '../store/useTradeStore';

export function GameTimer() {
  const { gameStatus, gameEndTime } = useTradeStore();
  const [timeLeft, setTimeLeft] = useState('10:00');

  useEffect(() => {
    if (gameStatus !== 'PLAYING' || !gameEndTime) {
      setTimeLeft('00:00');
      return;
    }

    // 每秒只做一件事：計算距離「死線」還有多少毫秒
    const interval = setInterval(() => {
      const now = Date.now();
      const distance = gameEndTime - now;

      if (distance <= 0) {
        setTimeLeft('00:00');
        clearInterval(interval);
        return;
      }

      const minutes = Math.floor((distance % (1000 * 60 * 60)) / (1000 * 60));
      const seconds = Math.floor((distance % (1000 * 60)) / 1000);

      // 補零格式化 (例如 09:05)
      setTimeLeft(`${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`);
    }, 1000);

    return () => clearInterval(interval);
  }, [gameStatus, gameEndTime]);

  // 根據不同狀態顯示不同顏色
  let displayColor = '#9ca3af'; // WAITING 灰色
  let displayText = '等待開局...';

  if (gameStatus === 'PLAYING') {
    // 剩下不到一分鐘時，數字變紅色警告！
    displayColor = timeLeft.startsWith('00:') ? '#ef4444' : '#10b981'; 
    displayText = `⚔️ 剩餘時間: ${timeLeft}`;
  } else if (gameStatus === 'ENDED') {
    displayColor = '#f59e0b'; // ENDED 橘黃色
    displayText = '🏆 結算中';
  }

  return (
    <div style={{ 
      fontSize: '1.5rem', 
      fontWeight: 'bold', 
      padding: '10px 25px', 
      backgroundColor: '#1e1e2f', 
      borderRadius: '8px', 
      border: `2px solid ${displayColor}`,
      color: displayColor,
      textAlign: 'center',
      minWidth: '200px'
    }}>
      {displayText}
    </div>
  );
}