import { useNavigate } from 'react-router-dom'

export default function NotFoundPage() {
  const navigate = useNavigate()
  return (
    <div style={{ display:'flex', flexDirection:'column', alignItems:'center', justifyContent:'center', height:'100vh', gap:16, padding:24, textAlign:'center' }}>
      <div style={{ width:64, height:64, background:'var(--bg2)', border:'1px solid var(--border)', borderRadius:16, display:'flex', alignItems:'center', justifyContent:'center', color:'var(--faint)' }}>
        <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="8" x2="12" y2="12"/>
          <line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
      </div>
      <div>
        <h2 style={{ fontSize:20, fontWeight:700, marginBottom:6 }}>Страница не найдена</h2>
        <p style={{ fontSize:14, color:'var(--muted)', marginBottom:24 }}>Запрошенная страница не существует</p>
      </div>
      <button className="btn-dark" onClick={() => navigate('/')}>
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="15,18 9,12 15,6"/></svg>
        На главную
      </button>
    </div>
  )
}
