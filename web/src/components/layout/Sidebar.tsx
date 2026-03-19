import { useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore, useUIStore } from '../../store'
import { ACTS_DATA } from '../../data'

interface Props { mobOpen?: boolean; onClose?: () => void }

export default function Sidebar({ mobOpen, onClose }: Props) {
  const navigate = useNavigate()
  const location = useLocation()
  const { currentUser, logout } = useAuthStore()
  const setModalShortcuts = useUIStore(s => s.setModalShortcuts)

  const isActive = (path: string) =>
    path === '/' ? location.pathname === '/' : location.pathname.startsWith(path)

  const go = (path: string) => { navigate(path); onClose?.() }
  const handleLogout = () => { logout(); navigate('/auth') }

  return (
    <aside className={`sidebar${mobOpen ? ' mob-open' : ''}`}>
      {/* Brand — click goes to home */}
      <div className="sb-brand" onClick={() => go('/')} title="На главную">
        <div className="sb-logo">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
            <line x1="9" y1="13" x2="15" y2="13"/>
            <line x1="9" y1="17" x2="12" y2="17"/>
          </svg>
        </div>
        <div>
          <span className="sb-name">legist</span>
          <span className="sb-tier">Enterprise</span>
        </div>
      </div>

      {/* Nav */}
      <nav className="sb-nav">
        <button className={`sb-link${isActive('/acts') ? ' active' : ''}`} onClick={() => go('/acts')}>
          Акты <span className="sb-badge">{ACTS_DATA.length}</span>
        </button>
        <button className={`sb-link${location.pathname === '/' ? ' active' : ''}`} onClick={() => go('/')}>
          Загрузить акты
        </button>
        <button className={`sb-link${isActive('/chain') ? ' active' : ''}`} onClick={() => go('/chain/1')}>
          Цепочка редакций
        </button>
        <button className={`sb-link${isActive('/assistant') ? ' active' : ''}`} onClick={() => go('/assistant')}>
          Ассистент <span className="sb-badge-dot"/>
        </button>

        <div className="sb-divider"/>

        <div className="sb-section-label">Проекты</div>

        <div style={{ padding:'6px 8px', fontSize:13, fontWeight:600, color:'var(--muted)', display:'flex', alignItems:'center', gap:6 }}>
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" style={{ opacity:.6 }}>
            <rect x="3" y="3" width="7" height="7" rx="0.5"/><rect x="14" y="3" width="7" height="7" rx="0.5"/>
            <rect x="14" y="14" width="7" height="7" rx="0.5"/><rect x="3" y="14" width="7" height="7" rx="0.5"/>
          </svg>
          Трудовое право
        </div>
        <button className="sb-link" style={{ paddingLeft:22, fontSize:'12.5px' }} onClick={() => go('/acts/1')}>
          ПВТР ООО ТехПром
        </button>
        <button className="sb-link" style={{ paddingLeft:22, fontSize:'12.5px' }}>
          Положение об ОТ
        </button>
        <button className="sb-link" style={{ fontSize:'12.5px', color:'var(--faint)', gap:6 }}>
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
            <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          Новый проект
        </button>
      </nav>

      {/* Keyboard hint */}
      <div className="sb-kbhint" onClick={() => setModalShortcuts(true)}>
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
          <rect x="2" y="4" width="20" height="16" rx="2"/>
          <path d="M6 8h.01M10 8h.01M14 8h.01M18 8h.01M8 12h.01M12 12h.01M16 12h.01M7 16h10"/>
        </svg>
        Горячие клавиши
        <kbd className="sb-kbd">?</kbd>
      </div>

      {/* User — click logs out */}
      <div className="sb-user" onClick={handleLogout} title="Нажмите для выхода">
        <div className="sb-ava">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor">
            <circle cx="12" cy="8" r="4"/><path d="M4 20c0-3.31 3.58-6 8-6s8 2.69 8 6"/>
          </svg>
        </div>
        <div className="sb-user-info">
          <span className="sb-uname">{currentUser.name || 'Пользователь'}</span>
          <span className="sb-uemail">{currentUser.email}</span>
        </div>
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" style={{ color:'var(--faint)', flexShrink:0 }}>
          <polyline points="9,18 15,12 9,6"/>
        </svg>
      </div>
    </aside>
  )
}
