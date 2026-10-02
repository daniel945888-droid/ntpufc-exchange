import { useEffect, useRef } from 'react';
import { createChart, ColorType } from 'lightweight-charts';

export function TradingChart() {
  const chartContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!chartContainerRef.current) return;

    // 1. 初始化圖表實體，設定深色主題
    const chart = createChart(chartContainerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: '#181818' },
        textColor: '#d1d4dc',
      },
      grid: {
        vertLines: { color: '#333' },
        horzLines: { color: '#333' },
      },
      width: chartContainerRef.current.clientWidth,
      height: 350,
      timeScale: {
        timeVisible: true,
        secondsVisible: false,
      },
    });

    // 2. 新增 K 線圖 (Candlestick) 序列
    const candlestickSeries = chart.addCandlestickSeries({
      upColor: '#26a69a',
      downColor: '#ef5350',
      borderVisible: false,
      wickUpColor: '#26a69a',
      wickDownColor: '#ef5350',
    });

    // 3. 注入初始測試資料 (時間, 開盤, 最高, 最低, 收盤)
    candlestickSeries.setData([
      { time: '2026-09-18', open: 90, high: 105, low: 85, close: 100 },
      { time: '2026-09-19', open: 100, high: 115, low: 95, close: 110 },
      { time: '2026-09-20', open: 110, high: 120, low: 105, close: 115 },
      { time: '2026-09-21', open: 115, high: 118, low: 90, close: 95 },
      { time: '2026-09-22', open: 95, high: 102, low: 88, close: 100 },
    ]);

    // 4. 處理視窗大小改變時的 RWD 自適應
    const handleResize = () => {
      chart.applyOptions({ width: chartContainerRef.current?.clientWidth });
    };
    window.addEventListener('resize', handleResize);

    // 清理函數：元件卸載時銷毀圖表
    return () => {
      window.removeEventListener('resize', handleResize);
      chart.remove();
    };
  }, []);

  return (
    <div 
      ref={chartContainerRef} 
      style={{ width: '100%', height: '350px', border: '1px solid #333', borderRadius: '8px', overflow: 'hidden' }} 
    />
  );
}