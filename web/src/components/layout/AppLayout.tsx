import { useState } from 'react'
import { Outlet, Navigate } from 'react-router-dom'
import Sidebar from './Sidebar'
import { useAuthStore, useUIStore } from '../../store'
import { useKeyboard } from '../../hooks/useKeyboard'

// ── AUTH GUARD ────────────────────────────────────────────
export function AuthGuard() {
  const isAuthenticated = useAuthStore(s => s.isAuthenticated)
  if (!isAuthenticated) return <Navigate to="/auth" replace/>
  return <Outlet/>
}

// ── APP LAYOUT ────────────────────────────────────────────
export function AppLayout() {
  const { toast, modalDel, modalDelText, modalNtsi, modalShortcuts,
    setModalDel, setModalNtsi, setModalShortcuts, showToast } = useUIStore()
  const [ntsiQuery, setNtsiQuery] = useState('')
  const [ntsiType, setNtsiType] = useState<'all'|'law'|'decree'|'post'|'code'>('all')
  const [mobOpen, setMobOpen] = useState(false)

  // Wire up keyboard shortcuts globally
  useKeyboard()

  const ntsiSearch = () => {
    if (!ntsiQuery.trim()) { showToast('Введите поисковый запрос'); return }
    window.open('https://pravo.by/search/?c=&text=' + encodeURIComponent(ntsiQuery) + '&q=&l=', '_blank', 'noopener')
    setModalNtsi(false)
  }

  return (
    <div className="app-layout">
      {/* Mobile header */}
      <div className="mob-header">
        <button className="mob-menu-btn" onClick={() => setMobOpen(!mobOpen)}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/>
          </svg>
        </button>
        <span className="mob-brand">legist</span>
      </div>

      {/* Mobile overlay */}
      {mobOpen && <div className="mob-overlay" onClick={() => setMobOpen(false)}/>}

      <Sidebar mobOpen={mobOpen} onClose={() => setMobOpen(false)}/>

      <main className="main">
        <Outlet/>
      </main>

      {/* ── Toast ── */}
      {toast.visible && <div className="toast">{toast.msg}</div>}

      {/* ── Modal: Delete ── */}
      {modalDel && (
        <div className="modal-overlay" onClick={() => setModalDel(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Удалить акт?</h3>
              <button className="modal-close" onClick={() => setModalDel(false)}>
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </button>
            </div>
            <p className="modal-text">{modalDelText}</p>
            <div className="modal-actions">
              <button className="btn-outline" onClick={() => setModalDel(false)}>Отмена</button>
              <button className="btn-danger" onClick={() => { setModalDel(false); showToast('✓ Акт удалён (требуется backend)') }}>Удалить</button>
            </div>
          </div>
        </div>
      )}

      {/* ── Modal: НЦПИ ── */}
      {modalNtsi && (
        <div className="modal-overlay" onClick={() => setModalNtsi(false)}>
          <div className="modal modal-wide" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Поиск по НЦПИ</h3>
              <button className="modal-close" onClick={() => setModalNtsi(false)}>
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </button>
            </div>
            <p className="modal-sub">Национальный правовой интернет-портал Республики Беларусь (pravo.by)</p>
            <div className="search-box" style={{ marginBottom:10 }}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
              <input className="search-input" autoFocus placeholder="Номер закона, статья, тема..."
                value={ntsiQuery} onChange={e => setNtsiQuery(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && ntsiSearch()}/>
            </div>
            <div className="filter-chips" style={{ marginBottom:14 }}>
              {([['all','Все'],['law','Законы'],['decree','Декреты / Указы'],['post','Постановления'],['code','Кодексы']] as const).map(([v,l]) => (
                <button key={v} className={`fchip${ntsiType===v?' active':''}`} onClick={() => setNtsiType(v)}>{l}</button>
              ))}
            </div>
            <button className="btn-dark" style={{ width:'100%', marginBottom:16 }} onClick={ntsiSearch}>
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
              Искать на НЦПИ
            </button>
            <div className="ntsi-quick">
              <p className="ntsi-quick-title">Часто используемые акты</p>
              {[
                ['ТК РБ — Трудовой кодекс','https://pravo.by/document/?guid=3871&p0=hk9900296'],
                ['НК РБ — Налоговый кодекс','https://pravo.by'],
                ['Конституция Республики Беларусь','https://pravo.by'],
                ['Закон №130-З о нормотворчестве','https://pravo.by'],
                ['Закон об архивном деле','https://pravo.by'],
                ['Закон об электронном документе','https://pravo.by'],
              ].map(([t,h]) => <a key={t} className="ntsi-link" href={h} target="_blank" rel="noopener">{t}</a>)}
            </div>
          </div>
        </div>
      )}

      {/* ── Modal: Shortcuts ── */}
      {modalShortcuts && (
        <div className="modal-overlay" onClick={() => setModalShortcuts(false)}>
          <div className="modal modal-kbd" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Горячие клавиши</h3>
              <button className="modal-close" onClick={() => setModalShortcuts(false)}>
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </button>
            </div>
            <div className="sc-cols">
              <div>
                <div className="sc-group">Навигация</div>
                {[['Список актов',['G','A']],['Главная',['G','H']],['Ассистент',['G','I']],['Цепочка',['G','C']]].map(([l,keys]) => (
                  <div key={l as string} className="sc-row">
                    <span>{l}</span>
                    <div className="sc-keys">{(keys as string[]).map(k => <kbd key={k}>{k}</kbd>)}</div>
                  </div>
                ))}
                <div className="sc-group" style={{ marginTop:14 }}>Интерфейс</div>
                {[['Это окно',['?']],['Закрыть',['Esc']]].map(([l,keys]) => (
                  <div key={l as string} className="sc-row">
                    <span>{l}</span>
                    <div className="sc-keys">{(keys as string[]).map(k => <kbd key={k}>{k}</kbd>)}</div>
                  </div>
                ))}
              </div>
              <div>
                <div className="sc-group">Действия</div>
                {[['Новое сравнение',['N']],['Поиск по НЦПИ',['Ctrl','K']],['Экспорт .docx',['Ctrl','E']]].map(([l,keys]) => (
                  <div key={l as string} className="sc-row">
                    <span>{l}</span>
                    <div className="sc-keys">{(keys as string[]).map(k => <kbd key={k}>{k}</kbd>)}</div>
                  </div>
                ))}
                <div className="sc-group" style={{ marginTop:14 }}>В таблице</div>
                {[['Все / Противоречия',['1','2']],['Проверить / Безопасно',['3','4']],['Плотный вид',['D']]].map(([l,keys]) => (
                  <div key={l as string} className="sc-row">
                    <span>{l}</span>
                    <div className="sc-keys">{(keys as string[]).map(k => <kbd key={k}>{k}</kbd>)}</div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
