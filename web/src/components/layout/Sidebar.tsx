import { useState, useEffect } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore, useUIStore } from '../../store'
import { ACTS_DATA } from '../../data'
import { apiFetch } from '../../utils/api'

interface Props { mobOpen?: boolean; onClose?: () => void }

export default function Sidebar({ mobOpen, onClose }: Props) {
  const navigate = useNavigate()
  const location = useLocation()
  const { currentUser, logout, isAuthenticated } = useAuthStore()
  const setModalShortcuts = useUIStore(s => s.setModalShortcuts)
  const [fileCount, setFileCount] = useState(0)

  useEffect(() => {
    if (!isAuthenticated) return
    const fetchCount = async () => {
      const token = localStorage.getItem('legist_token')
      if (!token) return
      try {
        const res = await apiFetch('/api/files', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (res.ok) {
          const data = await res.json()
          if (data.object === 'list') setFileCount(data.data.length)
        }
      } catch (err) {
        console.error('Sidebar fetch error:', err)
        setFileCount(0)
      }
    }
    fetchCount()
    // Обновляем раз в 30 секунд для актуальности
    const iv = setInterval(fetchCount, 30000)
    return () => clearInterval(iv)
  }, [isAuthenticated])

  const isActive = (path: string) =>
    path === '/' ? location.pathname === '/' : location.pathname.startsWith(path)

  const go = (path: string) => { navigate(path); onClose?.() }
  const handleLogout = () => { logout(); navigate('/auth') }

  return (
    <aside className={`sidebar${mobOpen ? ' mob-open' : ''}`}>
      {/* Brand — click goes to home */}
      <div className="sb-brand" onClick={() => go('/')} title="На главную">
        <div className="sb-logo">
          <img src="/stork.png" alt="legist" style={{ width: 22, height: 22, objectFit: 'cover' }} />
        </div>
        <div>
          <span className="sb-name">легист.бел</span>
          <span className="sb-tier">Легист.бел</span>
        </div>
      </div>

      {/* Nav */}
      <nav className="sb-nav">
        <button className={`sb-link${isActive('/acts') ? ' active' : ''}`} onClick={() => go('/acts')}>
          Акты <span className="sb-badge">{fileCount || 0}</span>
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
