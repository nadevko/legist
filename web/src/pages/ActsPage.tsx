// ─────────────────────────────────────────────────────────
// ACTS PAGE
// ─────────────────────────────────────────────────────────
import { useState, useMemo, useRef, useCallback, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useUIStore } from '../store'
import { pl, rBdg, rFull } from '../utils/helpers'
import { apiFetch } from '../utils/api'

export function ActsPage() {
  const navigate = useNavigate()
  const showToast = useUIStore(s => s.showToast)
  const [view, setView] = useState<'grid'|'list'>('grid')
  const [typeFilter, setTypeFilter] = useState<'all'|'ЛНА'|'НПА'>('all')
  const [search, setSearch] = useState('')
  const [searchInput, setSearchInput] = useState('')
  const [acts, setActs] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout>|null>(null)

  useEffect(() => {
    const fetchActs = async () => {
      setLoading(true)
      const token = localStorage.getItem('legist_token')
      if (!token) { setLoading(false); return }
      try {
        const res = await apiFetch('/api/files', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (res.ok) {
          const data = await res.json()
          if (data.object === 'list') {
            setActs(data.data.map((f: any) => ({
              id: f.id,
              type: 'ЛНА', // Бэкенд пока не хранит тип, ставим по умолчанию
              title: f.name || 'Без названия',
              org: 'Мои документы',
              versions: 1, // В текущей модели бэкенда 1 файл = 1 версия
              date: new Date(f.created * 1000).toLocaleDateString('ru-RU'),
              updatedBy: 'Вы'
            })))
          }
        }
      } catch (err) {
        console.error('Fetch acts error:', err)
        showToast('Ошибка загрузки файлов')
      } finally {
        setLoading(false)
      }
    }
    fetchActs()
  }, [])

  const handleSearch = useCallback((val: string) => {
    setSearchInput(val)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => setSearch(val), 200)
  }, [])
  const [ctxMenu, setCtxMenu] = useState<{x:number;y:number;id:string}|null>(null)

  const filtered = useMemo(() => {
    let f = acts
    if (typeFilter !== 'all') f = f.filter(a => a.type === typeFilter)
    const q = search.trim().toLowerCase()
    if (q) f = f.filter(a => a.title.toLowerCase().includes(q) || a.org.toLowerCase().includes(q))
    return f
  }, [acts, typeFilter, search])

  const showCtx = (e: React.MouseEvent, id: string) => {
    e.stopPropagation()
    setCtxMenu({ x: Math.min(e.clientX, window.innerWidth - 170), y: Math.min(e.clientY, window.innerHeight - 180), id })
  }

  return (
    <div className="page" onClick={() => setCtxMenu(null)}>
      <div className="page-topnav">
        <button className="topnav-btn active">Акты</button>
      </div>
      <div className="toolbar">
        <div className="search-box">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          <input className="search-input" placeholder="Поиск по названию, организации..." value={searchInput} onChange={e => handleSearch(e.target.value)}/>
        </div>
        <div className="filter-chips">
          {(['all','ЛНА','НПА'] as const).map(t => (
            <button key={t} className={`fchip${typeFilter===t?' active':''}`} onClick={() => setTypeFilter(t)}>
              {t === 'all' ? 'Все' : t}
            </button>
          ))}
        </div>
        <div className="toolbar-right">
          <button className="btn-outline" onClick={() => navigate('/')}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
            Добавить акт
          </button>
          <div className="view-switch">
            <button className={`vsw${view==='grid'?' active':''}`} onClick={() => setView('grid')}>
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/></svg>
            </button>
            <button className={`vsw${view==='list'?' active':''}`} onClick={() => setView('list')}>
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>
            </button>
          </div>
        </div>
      </div>

      {view === 'grid' ? (
        <div className="acts-grid">
          {filtered.map(a => (
            <div key={a.id} className="act-card" onClick={() => navigate('/acts/'+a.id)}>
              <div className="ac-top">
                <span className="ac-type">{a.type}</span>
                <span className="ac-vers-pill">{a.versions} {pl(a.versions,'версия','версии','версий')}</span>
              </div>
              <div className="ac-title">{a.title}</div>
              <div className="ac-org">{a.org}</div>
              <div className="ac-info-row">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" width="12" height="12"><rect x="3" y="4" width="18" height="18" rx="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>
                <span>{a.date}</span>
                <span style={{ margin:'0 3px', color:'var(--bg3)' }}>·</span>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" width="12" height="12"><circle cx="12" cy="8" r="4"/><path d="M4 20c0-3.31 3.58-6 8-6s8 2.69 8 6"/></svg>
                <span>{a.updatedBy}</span>
              </div>
              <div className="ac-footer">
                <button className="btn-outline" style={{ fontSize:12, padding:'5px 12px' }} onClick={e => { e.stopPropagation(); navigate('/acts/'+a.id) }}>Подробнее</button>
                <button className="dots-btn" onClick={e => showCtx(e, a.id)}>···</button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="acts-list">
          {filtered.map(a => (
            <div key={a.id} className="act-row" onClick={() => navigate('/acts/'+a.id)}>
              <div><span className="ac-type">{a.type}</span></div>
              <div style={{ flex:1, fontWeight:600, fontSize:'13.5px' }}>{a.title}</div>
              <div style={{ width:160, fontSize:12, color:'var(--muted)' }}>{a.org}</div>
              <div style={{ width:100, fontSize:12, color:'var(--faint)' }}>{a.date}</div>
              <div style={{ fontSize:12, color:'var(--faint)' }}>{a.versions} {pl(a.versions,'версия','версии','версий')}</div>
              <div style={{ fontSize:12, color:'var(--muted)' }}>{a.updatedBy}</div>
              <button className="dots-btn" onClick={e => showCtx(e, a.id)}>···</button>
            </div>
          ))}
        </div>
      )}

      <div className="acts-pagination">
        <span>Показано 1–{filtered.length} из {filtered.length}</span>
        <div className="pg-btns">
          <button className="pg-b" onClick={() => showToast('Пагинация (требуется backend)')}>‹</button>
          <button className="pg-b" onClick={() => showToast('Пагинация (требуется backend)')}>›</button>
        </div>
      </div>

      {/* Context menu */}
      {ctxMenu && (
        <div className="ctx-menu" style={{ left:ctxMenu.x, top:ctxMenu.y }}>
          <button className="ctx-item" onClick={() => { navigate('/acts/'+ctxMenu.id); setCtxMenu(null) }}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>Открыть
          </button>
          <button className="ctx-item" onClick={() => { navigate('/'); setCtxMenu(null) }}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><rect x="2" y="3" width="9" height="18" rx="1"/><rect x="13" y="3" width="9" height="18" rx="1"/></svg>Сравнить
          </button>
          <div className="ctx-divider"/>
          <button className="ctx-item ctx-danger" onClick={() => { showToast('✓ Удалено'); setCtxMenu(null) }}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><polyline points="3,6 5,6 21,6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>Удалить
          </button>
        </div>
      )}
    </div>
  )
}
