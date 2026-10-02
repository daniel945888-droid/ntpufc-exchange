import { useState } from 'react';
import { useNavigate } from 'react-router-dom';

export function LoginWall() {
  const [nickname, setNickname] = useState('');
  const navigate = useNavigate();

  const handleJoin = (e: React.FormEvent) => {
    e.preventDefault();
    if (!nickname.trim()) return;
    
    // ✨ 將玩家輸入的暱稱，透過 URL 參數帶給交易大廳
    navigate(`/trade?user_id=${encodeURIComponent(nickname.trim())}`);
  };

  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', backgroundColor: '#111827', color: '#fff', fontFamily: 'sans-serif' }}>
      <form 
        onSubmit={handleJoin} 
        style={{ backgroundColor: '#1f2937', padding: '40px', borderRadius: '12px', textAlign: 'center', maxWidth: '400px', width: '100%', boxShadow: '0 10px 15px -3px rgba(0, 0, 0, 0.1)' }}
      >
        <h1 style={{ fontSize: '2rem', fontWeight: 'bold', marginBottom: '10px', color: '#818cf8' }}>
          NTPUFC 交易所
        </h1>
        <p style={{ color: '#9ca3af', marginBottom: '30px' }}>請輸入你的玩家暱稱或學號</p>

        <input
          type="text"
          placeholder="例如：交易之神"
          value={nickname}
          onChange={(e) => setNickname(e.target.value)}
          maxLength={15}
          required
          style={{ width: '100%', padding: '12px', marginBottom: '24px', borderRadius: '6px', border: 'none', backgroundColor: '#e5e7eb', color: '#000', fontSize: '1rem', boxSizing: 'border-box' }}
        />

        <button
          type="submit"
          style={{ width: '100%', backgroundColor: '#4f46e5', color: '#fff', fontWeight: 'bold', padding: '12px', borderRadius: '6px', border: 'none', cursor: 'pointer', fontSize: '1.1rem' }}
        >
          🚀 進入交易大廳
        </button>
      </form>
    </div>
  );
}