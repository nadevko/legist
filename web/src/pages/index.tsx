import { useState, useMemo, useRef, useEffect } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ACTS_DATA, CHANGES_DATA, HIERARCHY_DATA, VIOLATIONS_DATA, CHAIN_VERSIONS_DATA } from '../data'
import { useUIStore, useCompareStore } from '../store'
import { useFileUpload, useCompareProgress, useAssistant } from '../hooks'
import { rBdg, rFull, pl, esc, PROGRESS_STEPS } from '../utils/helpers'
import { apiFetch } from '../utils/api'
import type { Version, Risk } from '../types'

// ─────────────────────────────────────────────────────────
// ACT DETAIL PAGE
// ─────────────────────────────────────────────────────────

export function ActDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const showToast = useUIStore(s => s.showToast)
  const setCompare = useCompareStore(s => s.setCompare)
  const [rows, setRows] = useState<Version[]>([])
  const [selected, setSelected] = useState<Version | null>(null)
  const [versionFilter, setVersionFilter] = useState<'Все' | 'Актуальные' | 'Архив'>('Все')
  const [verSearch, setVerSearch] = useState('')
  const [loading, setLoading] = useState(false)
  const progress = useCompareProgress()

  useEffect(() => {
    const fetchFiles = async () => {
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
            const mapped = data.data.map((f: any, i: number) => ({
              id: f.id,
              num: data.data.length - i,
              date: new Date(f.created * 1000).toLocaleDateString('ru-RU'),
              author: 'Вы',
              changes: f.status === 'done' ? 5 : 0, // Мок, так как бэкенд пока не возвращает кол-во изменений
              size: Math.round(f.size / 1024) + ' КБ',
              status: f.status === 'done' ? (i === 0 ? 'Актуальная' : 'Архив') : 'Загрузка',
              checked: false
            }))
            setRows(mapped)
          }
        }
      } catch (err) {
        console.error('Fetch files error:', err)
        showToast('⚠ Ошибка загрузки файлов')
      }
    }
    fetchFiles()
  }, [])

  const filteredRows = useMemo(() => {
    let r = rows
    if (versionFilter === 'Актуальные') r = r.filter(v => v.status === 'Актуальная')
    if (versionFilter === 'Архив') r = r.filter(v => v.status === 'Архив')
    if (verSearch.trim()) r = r.filter(v => v.author?.toLowerCase().includes(verSearch.toLowerCase()))
    return r
  }, [rows, versionFilter, verSearch])

  const currentAct = rows.length > 0 ? { title: rows[0].author === 'Вы' ? 'Загруженный документ' : 'Документ', org: 'Мои документы' } : { title: 'Документ', org: '—' }
  const checked = rows.filter(v => v.checked)

  const toggleRow = (num: number) => setRows(rs => rs.map(r => r.num === num ? { ...r, checked: !r.checked } : r))
  const selectAll = (e: React.ChangeEvent<HTMLInputElement>) => setRows(rs => rs.map(r => ({ ...r, checked: e.target.checked })))
  const selectVer = (v: Version) => setSelected(s => s?.num === v.num ? null : v)

  const handleDelete = async (e: React.MouseEvent, v: Version) => {
    e.stopPropagation()
    if (!v.id) return
    if (!window.confirm(`Вы уверены, что хотите удалить ${v.num} версию?`)) return

    const token = localStorage.getItem('legist_token')
    try {
      const res = await apiFetch(`/api/files/${v.id}`, {
        method: 'DELETE',
        headers: { 
          'Authorization': `Bearer ${token}`,
          'Idempotency-Key': crypto.randomUUID(),
        }
      })
      if (res.ok) {
        showToast('✓ Файл удален')
        setRows(rs => rs.filter(r => r.id !== v.id))
        if (selected?.id === v.id) setSelected(null)
      } else {
        const data = await res.json()
        showToast('⚠ ' + (data?.error?.message || 'Ошибка удаления'))
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Ошибка сети'
      showToast('⚠ ' + msg)
    }
  }

  const startCompare = () => {
    if (checked.length !== 2) { showToast('⚠ Выберите ровно 2 версии для сравнения'); return }
    const [a, b] = [...checked].sort((x, y) => x.num - y.num)
    setCompare('Анализ изменений', (currentAct?.title || 'Документ') + ' · Версия ' + a.num + ' → Версия ' + b.num + ' · ' + a.date + ' vs ' + b.date)
    progress.start(id || 'temp', () => {
      setRows(rs => rs.map(r => ({ ...r, checked: false })))
      navigate('/compare/1')
    })
  }

  const changeRows = useMemo(() => selected ? CHANGES_DATA.slice(0, selected.changes) : [], [selected])
  const summary = useMemo(() => ({
    red: changeRows.filter((r: any) => r.risk === 'red').length,
    orange: changeRows.filter((r: any) => r.risk === 'orange').length,
    green: changeRows.filter((r: any) => r.risk === 'green').length,
  }), [changeRows])

  if (loading && rows.length === 0) return <div className="page" style={{ padding: 32, textAlign: 'center' }}>Загрузка акта...</div>

  return (
    <div className="page">
      <div className="page-topnav">
        <button className="topnav-btn" onClick={() => navigate('/acts')}>
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><polyline points="15,18 9,12 15,6" /></svg>
          Назад
        </button>
      </div>
      <div className="detail-header">
        <div>
          <h1 className="detail-title">{currentAct.title}</h1>
          <p className="detail-sub">{currentAct.org} · {rows.length} {pl(rows.length, 'версия', 'версии', 'версий')}</p>
        </div>
        <div className="detail-header-actions">
          <button className="btn-dark" onClick={() => navigate('/compare/1')}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="2" y="3" width="9" height="18" rx="1" /><rect x="13" y="3" width="9" height="18" rx="1" /></svg>
            Сравнить версии
          </button>
          <button className="btn-outline" onClick={() => navigate('/chain/' + id)}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="22,12 18,12 15,21 9,3 6,12 2,12" /></svg>
            Цепочка редакций
          </button>
        </div>
      </div>

      <div className="toolbar" style={{ marginBottom: 12 }}>
        <div className="search-box">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><circle cx="11" cy="11" r="8" /></svg>
          <input className="search-input" placeholder="Фильтровать версии..." value={verSearch} onChange={e => setVerSearch(e.target.value)} />
        </div>
        <div className="filter-chips">
          {(['Все', 'Актуальные', 'Архив'] as const).map(f => (
            <button key={f} className={`fchip${versionFilter === f ? ' active' : ''}`} onClick={() => setVersionFilter(f)}>{f}</button>
          ))}
        </div>
        <button
          disabled={checked.length !== 2}
          style={{ marginLeft: 'auto', opacity: checked.length !== 2 ? 0.45 : 1, cursor: checked.length !== 2 ? 'not-allowed' : 'pointer', background: 'var(--txt)', color: '#fff', border: 'none', borderRadius: 'var(--r)', padding: '7px 14px', fontFamily: 'var(--font)', fontSize: 13, fontWeight: 500, display: 'inline-flex', alignItems: 'center', gap: 6 }}
          onClick={startCompare}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="2" y="3" width="9" height="18" rx="1" /><rect x="13" y="3" width="9" height="18" rx="1" /></svg>
          Сравнить выбранные
          {checked.length > 0 && <span style={{ background: 'rgba(255,255,255,.2)', borderRadius: 10, padding: '1px 7px', fontSize: 11, marginLeft: 2 }}>{checked.length}/2</span>}
        </button>
      </div>

      <div className="versions-tbl">
        {/* Заголовок — точно как на скрине */}
        <div className="vtbl-head">
          <div>
            <input type="checkbox"
              onChange={selectAll}
              checked={filteredRows.length > 0 && filteredRows.every(v => v.checked)}
              style={{ width: 16, height: 16, cursor: 'pointer', accentColor: 'var(--txt)' }} />
          </div>
          <div>Версия</div>
          <div>Дата</div>
          <div>Автор</div>
          <div>Изменений</div>
          <div>Размер</div>
          <div>Статус</div>
          <div style={{ width: 40 }}></div>
        </div>
        {filteredRows.map(v => (
          <div key={v.num}
            className={`vtbl-row${selected?.num === v.num ? ' selected' : ''}${v.checked ? ' checked' : ''}`}
            onClick={() => selectVer(v)}>
            {/* Чекбокс — отдельная колонка слева */}
            <div onClick={e => e.stopPropagation()}>
              <input type="checkbox" checked={v.checked} onChange={() => toggleRow(v.num)} />
            </div>
            <div className="vtbl-ver">{v.num}</div>
            <div style={{ color: 'var(--txt)' }}>{v.date}</div>
            <div style={{ color: 'var(--txt)' }}>{v.author}</div>
            <div className="vtbl-changes">{v.changes > 0 ? v.changes + ' изм.' : '—'}</div>
            <div className="vtbl-size">{v.size}</div>
            <div>
              <span className={`vtbl-status ${v.status === 'Актуальная' ? 'status-act' : 'status-arch'}`}>
                {v.status}
              </span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'center' }}>
              <button 
                className="action-btn-del" 
                title="Удалить версию"
                onClick={(e) => handleDelete(e, v)}
                style={{ background: 'none', border: 'none', color: 'var(--muted)', cursor: 'pointer', padding: 4, borderRadius: 4, display: 'flex', alignItems: 'center', transition: 'all .2s' }}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path><line x1="10" y1="11" x2="10" y2="17"></line><line x1="14" y1="11" x2="14" y2="17"></line></svg>
              </button>
            </div>
          </div>
        ))}
      </div>

      {/* Version changes */}
      {selected && !progress.running && (
        <div style={{ margin: '20px 20px 0' }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
            <div>
              <span style={{ fontSize: 15, fontWeight: 700 }}>Версия {selected.num}</span>
              <span style={{ fontSize: 13, color: 'var(--muted)', marginLeft: 10 }}>{selected.date} · {selected.author}</span>
            </div>
            <div style={{ display: 'flex', gap: 8 }}>
              <button className="btn-outline" style={{ fontSize: 12, padding: '5px 12px' }} onClick={() => showToast('📄 Экспорт .docx')}>Экспорт .docx</button>
              <button className="btn-dark" style={{ fontSize: 12, padding: '5px 12px' }} onClick={() => navigate('/compare/1')}>
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="2" y="3" width="9" height="18" rx="1" /><rect x="13" y="3" width="9" height="18" rx="1" /></svg>
                Полный анализ
              </button>
              <button className="dots-btn" onClick={() => setSelected(null)} style={{ fontSize: 18, padding: '4px 8px' }}>×</button>
            </div>
          </div>
          {selected.changes > 0 && (
            <>
              <div style={{ display: 'flex', gap: 8, marginBottom: 14 }}>
                {[{ k: 'red', label: 'Противоречий', cls: 'chk-r', val: summary.red }, { k: 'orange', label: 'Проверить', cls: 'chk-o', val: summary.orange }, { k: 'green', label: 'Безопасно', cls: 'chk-g', val: summary.green }].map(s => (
                  <div key={s.k} className={`chk-card ${s.cls}`} style={{ minWidth: 80, textAlign: 'center' }}>
                    <div className="chk-num">{s.val}</div>
                    <div style={{ fontSize: 11, color: 'var(--muted)' }}>{s.label}</div>
                  </div>
                ))}
              </div>
              <div className="tbl-wrap">
                <table className="ctbl">
                  <thead><tr>
                    <th style={{ width: 36 }}>#</th><th>Раздел</th><th>Было</th><th>Стало</th><th>Тип изменения</th><th>Риск</th><th>Закон</th><th>Рекомендация</th>
                  </tr></thead>
                  <tbody>
                    {changeRows.map((r: any) => (
                      <tr key={r.n}>
                        <td style={{ color: 'var(--faint)', fontSize: 12 }}>{r.n}</td>
                        <td className="td-s">{r.s}</td>
                        <td className="td-o"><span title={r.old}>{r.old}</span></td>
                        <td className="td-n"><span title={r.nw}>{r.nw}</span></td>
                        <td className="td-t">{r.type}</td>
                        <td><span className={`bdg ${rBdg(r.risk)}`}>{rFull(r.risk)}</span></td>
                        <td className="td-m" style={{ fontSize: '11.5px' }}>{r.law}</td>
                        <td className="td-r"><span title={r.rec}>{r.rec}</span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
          {selected.changes === 0 && (
            <div style={{ padding: 32, textAlign: 'center', color: 'var(--faint)', background: 'var(--white)', border: '1px solid var(--border)', borderRadius: 'var(--r2)' }}>
              Изменений не зафиксировано — первоначальная редакция
            </div>
          )}
        </div>
      )}

      {/* Progress */}
      {progress.running && (
        <div style={{ margin: '16px 20px 0' }}>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16, background: 'var(--white)', border: '1px solid var(--border)', borderRadius: 'var(--r2)', padding: '20px 24px' }}>
            <div style={{ width: 40, height: 40, background: 'var(--bg2)', borderRadius: 10, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--muted)', flexShrink: 0 }}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><polyline points="22,12 18,12 15,21 9,3 6,12 2,12" /></svg>
            </div>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 12 }}>{progress.label}</div>
              <div className="progress-bar" style={{ marginBottom: 10 }}>
                <div className="progress-fill" style={{ width: progress.pct + '%' }} />
              </div>
              <div className="progress-steps">
                {progress.steps.map((s, i) => (
                  <span key={s} className={`ps${progress.step === i ? ' active' : progress.step > i ? ' done' : ''}`}>{s}</span>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// ─────────────────────────────────────────────────────────
// HOME PAGE (Upload + Recent)
// ─────────────────────────────────────────────────────────
export function HomePage() {
  const navigate = useNavigate()
  const showToast = useUIStore(s => s.showToast)
  const { oldFile, newFile, dzOldOver, dzNewOver, loading, setDzOldOver, setDzNewOver, onDrop, onSelect, removeOld, removeNew } = useFileUpload()
  const progress = useCompareProgress()
  const [recent, setRecent] = useState<any[]>([])
  const fiOldRef = useRef<HTMLInputElement>(null)
  const fiNewRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const fetchRecent = async () => {
      const token = localStorage.getItem('legist_token')
      if (!token) return
      try {
        const res = await apiFetch('/api/files?limit=3', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (res.ok) {
          const data = await res.json()
          if (data.object === 'list') {
            setRecent(data.data.map((f: any) => ({
              id: f.id,
              title: f.filename,
              org: 'Мои загрузки',
              v1: '—',
              v2: 'v1',
              date: new Date(f.created * 1000).toLocaleDateString('ru-RU'),
              risk: f.status === 'done' ? 'green' : 'orange'
            })))
          }
        }
      } catch (err) {
        console.error('Recent fetch error:', err)
      }
    }
    fetchRecent()
  }, [])

  const startCompare = () => {
    if (!oldFile || !newFile) { showToast('⚠ Загрузите оба файла для сравнения'); return }
    if (!newFile.id) { showToast('⚠ Дождитесь завершения загрузки файла'); return }
    progress.start(newFile.id, () => navigate('/compare/new'))
  }

  const recentComparisons = recent.length > 0 ? recent : [
    { id: 1, title: 'Правила внутреннего трудового распорядка', org: 'ООО «ТехПром»', v1: 'v4', v2: 'v5', date: '12.02.2025', risk: 'red' as Risk },
    { id: 2, title: 'Положение об оплате труда', org: 'ООО «ТехПром»', v1: 'v2', v2: 'v3', date: '05.11.2024', risk: 'orange' as Risk },
  ]

  const FileCard = ({ side, file, dzOver, setOver }: { side: 'old' | 'new', file: typeof oldFile, dzOver: boolean, setOver: (v: boolean) => void }) => (
    <div className="upload-card">
      <div className="uc-hdr"><h3>{side === 'old' ? 'Старая версия' : 'Новая версия'}</h3><p>{side === 'old' ? 'Загрузите предыдущую редакцию' : 'Загрузите обновлённую редакцию документа'}</p></div>
      {!file ? (
        <div className={`dz${dzOver ? ' over' : ''}`}
          onDragOver={e => { e.preventDefault(); setOver(true) }}
          onDragLeave={() => setOver(false)}
          onDrop={e => onDrop(e, side)}
          onClick={() => (side === 'old' ? fiOldRef : fiNewRef).current?.click()}>
          <div className="dz-icon">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><polyline points="16,16 12,12 8,16" /><line x1="12" y1="12" x2="12" y2="21" /><path d="M20.39 18.39A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3" /></svg>
          </div>
          <p className="dz-text">Перетащите файл или нажмите для поиска</p>
          <p className="dz-hint">Только DOCX и PDF файлы</p>
          <button className="dz-btn" onClick={e => { e.stopPropagation(); (side === 'old' ? fiOldRef : fiNewRef).current?.click() }}>Смотреть файлы</button>
          <input ref={side === 'old' ? fiOldRef : fiNewRef} type="file" accept=".docx,.pdf" style={{ display: 'none' }} onChange={e => onSelect(e, side)} />
        </div>
      ) : (
        <div className="fp">
          <div className="fp-ico"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14,2 14,8 20,8" /></svg></div>
          <div><div className="fp-name">{file.name}</div><div className="fp-size">{Math.round(file.size / 1024)} KB</div></div>
          <button className="fp-rm" onClick={side === 'old' ? removeOld : removeNew}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
          </button>
        </div>
      )}
    </div>
  )

  return (
    <div className="page">
      <div style={{ padding: '32px 32px 0', maxWidth: 900, margin: '0 auto', width: '100%' }}>
        <h1 style={{ fontSize: 22, fontWeight: 700, marginBottom: 6 }}>Сравнение редакций НПА</h1>
        <p style={{ fontSize: '13.5px', color: 'var(--muted)', marginBottom: 28, lineHeight: 1.6 }}>
          Загрузите две версии нормативного акта — AI выявит структурные и семантические изменения и проверит соответствие законодательству Республики Беларусь
        </p>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 40px 1fr', alignItems: 'start', gap: 0, marginBottom: 16 }}>
          <FileCard side="old" file={oldFile} dzOver={dzOldOver} setOver={setDzOldOver} />
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', paddingTop: 72 }}>
            <div style={{ width: 32, height: 32, border: '1px solid var(--border)', borderRadius: '50%', background: 'var(--white)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--faint)' }}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="5" y1="12" x2="19" y2="12" /><polyline points="12,5 19,12 12,19" /></svg>
            </div>
          </div>
          <FileCard side="new" file={newFile} dzOver={dzNewOver} setOver={setDzNewOver} />
        </div>

        <button className="btn-compare" disabled={progress.running} onClick={startCompare}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polyline points="22,12 18,12 15,21 9,3 6,12 2,12" /></svg>
          Сравнить редакции
        </button>

        {progress.running && (
          <div className="progress-wrap">
            <div className="progress-label">{progress.label}</div>
            <div className="progress-bar"><div className="progress-fill" style={{ width: progress.pct + '%' }} /></div>
            <div className="progress-steps">
              {progress.steps.map((s, i) => <span key={s} className={`ps${progress.step === i ? ' active' : progress.step > i ? ' done' : ''}`}>{s}</span>)}
            </div>
          </div>
        )}

        {!progress.running && (
          <div style={{ marginTop: 36 }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: .6, marginBottom: 12 }}>Последние сравнения</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {recentComparisons.map(r => (
                <div key={r.id} onClick={() => navigate('/compare/1')}
                  style={{ display: 'flex', alignItems: 'center', gap: 14, background: 'var(--white)', border: '1px solid var(--border)', borderRadius: 'var(--r)', padding: '12px 16px', cursor: 'pointer', transition: 'border-color var(--t)' }}
                  onMouseEnter={e => (e.currentTarget as HTMLElement).style.borderColor = 'var(--bord2)'}
                  onMouseLeave={e => (e.currentTarget as HTMLElement).style.borderColor = 'var(--border)'}>
                  <div style={{ width: 32, height: 32, background: 'var(--bg2)', borderRadius: 7, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--muted)', flexShrink: 0 }}>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><rect x="2" y="3" width="9" height="18" rx="1" /><rect x="13" y="3" width="9" height="18" rx="1" /></svg>
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: '13.5px', fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{r.title}</div>
                    <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 2 }}>{r.org} · {r.v1} → {r.v2}</div>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexShrink: 0 }}>
                    <span style={{ fontSize: '11.5px', color: 'var(--faint)' }}>{r.date}</span>
                    <span className={`bdg ${rBdg(r.risk)}`}>{r.risk === 'red' ? 'Противоречия' : r.risk === 'orange' ? 'Проверить' : 'Безопасно'}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// ─────────────────────────────────────────────────────────
// COMPARE PAGE
// ─────────────────────────────────────────────────────────
const VTV_OLD = `
  <h4 style="font-size:14px;font-weight:700;text-align:center;margin-bottom:16px">Статья 218. Стандартные налоговые вычеты</h4>
  <p style="margin-bottom:8px">4) налоговый вычет за каждый месяц налогового периода распространяется на родителя,
  <span class="vdel" data-risk="red" data-sec="П.2.4" data-type="Изменение обязанности" data-law="ст.55 ТК РБ" data-rec="Проверить">опекуна, попечителя,</span>
  на обеспечении которых находится ребёнок:</p>
  <p style="margin-bottom:8px">1 <span class="vdel" data-risk="orange" data-sec="П.4.1" data-type="Изменение срока" data-law="Закон №433-З" data-rec="Проверить срок">000</span> рублей на первого ребёнка;</p>
  <p style="margin-bottom:8px"><span class="vdel" data-risk="red" data-sec="Раздел III" data-type="Удаление раздела" data-law="Закон №130-З" data-rec="Проверить">3 000 рублей на каждого ребёнка-инвалида до 18 лет</span></p>`

const VTV_NEW = `
  <h4 style="font-size:14px;font-weight:700;text-align:center;margin-bottom:16px">Статья 218. Стандартные налоговые вычеты</h4>
  <p style="margin-bottom:8px">4) налоговый вычет за каждый месяц распространяется на родителя, на обеспечении которых находится ребёнок:</p>
  <p style="margin-bottom:8px">1 <span class="vadd" data-risk="green" data-sec="П.4.1" data-type="Изменение суммы" data-law="—" data-rec="Повышает прозрачность">400</span> рублей на первого ребёнка;</p>
  <p style="margin-bottom:8px">12 000 рублей на каждого ребёнка-инвалида;</p>
  <p style="margin-bottom:8px"><span class="vadd" data-risk="green" data-sec="Статья 12" data-type="Добавление субъекта" data-law="—" data-rec="Проверить распределение">налоговый вычет распространяется на опекуна, попечителя:</span></p>`

export function ComparePage() {
  const navigate = useNavigate()
  const showToast = useUIStore(s => s.showToast)
  const setModalNtsi = useUIStore(s => s.setModalNtsi)
  const { title, sub } = useCompareStore()
  const [tab, setTab] = useState<'table' | 'vtv' | 'check'>('table')
  const [search, setSearch] = useState('')
  const [riskFilter, setRiskFilter] = useState<'all' | 'red' | 'orange' | 'green'>('all')
  const [dense, setDense] = useState(false)
  const [perPage, setPerPage] = useState(10)
  const [sortCol, setSortCol] = useState<'type' | 'risk' | null>(null)
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')
  const [vtvBar, setVtvBar] = useState<null | { risk: Risk; sec: string; type: string; law: string; rec: string }>(null)
  const [showRiskMenu, setShowRiskMenu] = useState(false)
  const [selectedRows, setSelectedRows] = useState<Set<number>>(new Set())
  const [activeChange, setActiveChange] = useState<typeof CHANGES_DATA[0] | null>(null)

  const handleSort = (col: 'type' | 'risk') => {
    if (sortCol === col) {
      setSortDir(d => d === 'asc' ? 'desc' : 'asc')
    } else {
      setSortCol(col)
      setSortDir('asc')
    }
  }

  // Highlight search matches
  const hl = (text: string, query: string): React.ReactNode => {
    if (!query.trim()) return text
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const re = new RegExp('(' + escaped + ')', 'gi')
    const parts = text.split(re)
    return <>{parts.map((p, i) => re.test(p) ? <mark key={i} className="highlight">{p}</mark> : p)}</>
  }

  // Real CSV export — no backend needed
  const exportCSV = (data: typeof CHANGES_DATA) => {
    const headers = ['№', 'Раздел', 'Было', 'Стало', 'Тип изменения', 'Риск', 'Закон', 'Рекомендация']
    const rows = data.map((r: any) => [r.n, r.s, r.old, r.nw, r.type, rFull(r.risk), r.law, r.rec]
      .map(v => '"' + String(v).replace(/"/g, '""') + '"').join(','))
    const csv = '\uFEFF' + headers.join(',') + '\n' + rows.join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a'); a.href = url; a.download = 'changes.csv'; a.click()
    URL.revokeObjectURL(url)
    showToast('✓ Файл changes.csv скачан')
  }

  const filtered = useMemo(() => {
    let f = [...CHANGES_DATA]
    if (riskFilter !== 'all') f = f.filter(r => r.risk === riskFilter)
    const q = search.toLowerCase().trim()
    if (q) f = f.filter(r =>
      r.s.toLowerCase().includes(q) ||
      r.old.toLowerCase().includes(q) ||
      r.nw.toLowerCase().includes(q) ||
      r.type.toLowerCase().includes(q) ||
      r.law.toLowerCase().includes(q)
    )
    if (sortCol) {
      const ord: Record<string, number> = { red: 0, orange: 1, green: 2 }
      f = f.sort((a, b) => {
        const va = sortCol === 'risk' ? ord[a.risk] : a.type.toLowerCase()
        const vb = sortCol === 'risk' ? ord[b.risk] : b.type.toLowerCase()
        if (va === vb) return 0
        return sortDir === 'asc' ? (va > vb ? 1 : -1) : (va < vb ? 1 : -1)
      })
    }
    return f
  }, [riskFilter, search, sortCol, sortDir])

  const redCount = useMemo(() => filtered.filter(r => r.risk === 'red').length, [filtered])
  const summary = redCount + ' высоких приоритета · ' + filtered.length + ' изменений'

  const handleVtvHover = (e: React.MouseEvent<HTMLDivElement>) => {
    const el = (e.target as Element).closest('[data-risk]') as HTMLElement | null
    if (!el) return
    setVtvBar({ risk: el.dataset.risk as Risk, sec: el.dataset.sec || '', type: el.dataset.type || '', law: el.dataset.law || '', rec: el.dataset.rec || '' })
  }

  return (
    <div className="page" onClick={() => { setVtvBar(null); setShowRiskMenu(false) }}>
      <div className="cmp-header">
        <div>
          <h1 className="cmp-title">{title}</h1>
          <p className="cmp-sub">{sub}</p>
        </div>
        <div className="cmp-header-actions">
          <button className="btn-outline" onClick={() => showToast('📄 Экспорт .docx (требуется backend)')}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="7,10 12,15 17,10" /><line x1="12" y1="15" x2="12" y2="3" /></svg>
            Экспорт .docx
          </button>
          <button className="btn-outline" onClick={() => setModalNtsi(true)}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" /></svg>
            Поиск по НЦПИ
          </button>
        </div>
      </div>

      <div className="tabs-bar">
        {[{ k: 'table', label: 'Сравнение таблица' }, { k: 'vtv', label: 'Сравнение vtv' }, { k: 'check', label: 'Проверка с отчётом' }].map(t => (
          <button key={t.k} className={`tab${tab === t.k ? ' active' : ''}`} onClick={() => setTab(t.k as typeof tab)}>{t.label}</button>
        ))}
      </div>

      {tab === 'table' && (
        <div className="tab-pane">
          <div className="tbl-toolbar">
            <div className="search-box sm" style={{ minWidth: 180 }}>
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" /></svg>
              <input className="search-input" placeholder="Фильтровать..." value={search} onChange={e => setSearch(e.target.value)} />
            </div>

            <div style={{ marginLeft: 'auto', display: 'flex', gap: 8 }}>
              <button className="btn-outline" style={{ display: 'flex', alignItems: 'center', gap: 5 }}
                onClick={() => exportCSV(filtered)}>
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" /><polyline points="7,10 12,15 17,10" /><line x1="12" y1="15" x2="12" y2="3" /></svg>
                CSV
              </button>
            </div>
          </div>
          {selectedRows.size > 0 && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '8px 12px', background: 'var(--bg2)', borderRadius: 'var(--r)', marginBottom: 8, fontSize: 13 }}>
              <span style={{ fontWeight: 600 }}>{selectedRows.size} выбрано</span>
              <button className="btn-outline" style={{ padding: '3px 10px', fontSize: 12 }}
                onClick={() => setSelectedRows(new Set())}>Снять выбор</button>
              <button className="btn-dark" style={{ padding: '3px 10px', fontSize: 12 }}
                onClick={() => { exportCSV(filtered.filter(r => selectedRows.has(r.n))); setSelectedRows(new Set()) }}>
                Экспорт выбранных
              </button>
            </div>
          )}
          <div className="tbl-wrap">
            <table className="ctbl">
              <colgroup>
                <col className="col-num" />
                <col className="col-section" />
                <col className="col-was" />
                <col className="col-now" />
                <col className="col-type" />
                <col className="col-risk" />
                <col className="col-law" />
                <col className="col-rec" />
              </colgroup>
              <thead><tr>
                <th style={{ width: 40 }}>№</th>
                <th style={{ width: 90 }}>Раздел</th>
                <th>Было</th>
                <th>Стало</th>
                <th className="sortable" style={{ width: 160 }} onClick={() => handleSort('type')}>
                  Тип изменения
                  {sortCol === 'type' && (
                    <span style={{ color: 'var(--txt)', fontSize: 10, marginLeft: 4 }}>
                      {sortDir === 'asc' ? '↑' : '↓'}
                    </span>
                  )}
                </th>
                <th className="sortable" style={{ width: 110 }} onClick={() => handleSort('risk')}>
                  Риск
                  {sortCol === 'risk' && (
                    <span style={{ color: 'var(--txt)', fontSize: 10, marginLeft: 4 }}>
                      {sortDir === 'asc' ? '↑' : '↓'}
                    </span>
                  )}
                </th>
                <th style={{ width: 140 }}>Закон</th>
                <th>Рекомендация</th>
              </tr></thead>
              <tbody>
                {filtered.map(r => (
                  <tr key={r.n}
                    className={selectedRows.has(r.n) ? 'selected' : ''}
                    onClick={() => setActiveChange(r)}
                    style={{ cursor: 'pointer' }}>
                    <td style={{ color: 'var(--faint)', fontSize: 12 }}>{r.n}</td>
                    <td className="td-s">{r.s}</td>
                    <td className="td-o"><span title={r.old}>{hl(r.old, search)}</span></td>
                    <td className="td-n"><span title={r.nw}>{hl(r.nw, search)}</span></td>
                    <td className="td-t">{r.type}</td>
                    <td><span className={`bdg ${rBdg(r.risk)}`}>{rFull(r.risk)}</span></td>
                    <td className="td-m" style={{ fontSize: '11.5px' }}>{r.law}</td>
                    <td className="td-r"><span title={r.rec}>{hl(r.rec, search)}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="tbl-foot">
            <div className="tbl-foot-summary">
              <strong>{redCount}</strong> высоких приоритета &nbsp;·&nbsp; {filtered.length} изменений
            </div>
            <div className="tbl-pg">
              <span className="pg-label">Строк на страницу</span>
              <select className="pg-sel" value={perPage} onChange={e => setPerPage(Number(e.target.value))}>
                <option value={10}>10</option>
                <option value={25}>25</option>
                <option value={50}>50</option>
              </select>
              <div className="pg-sep" />
              <span className="pg-label">Страница 1 из {Math.ceil(filtered.length / perPage) || 1}</span>
              <div style={{ display: 'flex', gap: 2 }}>
                {[['«', 'first'], ['‹', 'prev'], ['›', 'next'], ['»', 'last']].map(([label]) => (
                  <button key={label} className="pg-b" onClick={() => showToast('Пагинация (требуется backend)')}>{label}</button>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}

      {tab === 'vtv' && (
        <div className="tab-pane">
          <div className="vtv-wrap" onMouseOver={handleVtvHover} onMouseLeave={() => setVtvBar(null)}>
            <div className="vtv-col">
              <div className="vtv-col-hdr"><span className="vtv-col-lbl">Старая редакция</span><span className="vtv-col-ver">v1.0 · 12.02.2024</span></div>
              <div className="vtv-body" dangerouslySetInnerHTML={{ __html: VTV_OLD }} />
            </div>
            <div className="vtv-divider" />
            <div className="vtv-col">
              <div className="vtv-col-hdr"><span className="vtv-col-lbl">Новая редакция</span><span className="vtv-col-ver">v2.0 · 12.02.2025</span></div>
              <div className="vtv-body" dangerouslySetInnerHTML={{ __html: VTV_NEW }} />
            </div>
          </div>
          <div className={`vtv-bar${vtvBar ? ' visible' : ''}`}>
            <div className="vtv-bar-body">
              <div className="vtv-bar-row">
                <div><span className="vbl">Раздел</span><span className="vbv">{vtvBar?.sec || '—'}</span></div>
                <div><span className="vbl">Тип изменения</span><span className="vbv">{vtvBar?.type || '—'}</span></div>
                <div><span className="vbl">Закон</span><span className="vbv">{vtvBar?.law || '—'}</span></div>
              </div>
              <div><span className="vbl">Рекомендация</span><span className="vbv">{vtvBar?.rec || '—'}</span></div>
            </div>
            <div className="vtv-bar-right">
              {vtvBar && <span className={`bdg ${rBdg(vtvBar.risk)}`}>{rFull(vtvBar.risk)}</span>}
              <a href="https://pravo.by" target="_blank" rel="noopener" style={{ fontSize: 12, color: 'var(--muted)', textDecoration: 'none', borderBottom: '1px solid var(--border)' }}>pravo.by ↗</a>
            </div>
          </div>
        </div>
      )}

      {tab === 'check' && (
        <div className="tab-pane">
          <div className="chk-stats">
            <div className="chk-card chk-r"><div className="chk-num">3</div><div className="chk-lbl">Противоречия</div><div className="chk-desc">Требуют немедленного исправления</div></div>
            <div className="chk-card chk-o"><div className="chk-num">4</div><div className="chk-lbl">Требуют проверки</div><div className="chk-desc">Рекомендуется проверить соответствие</div></div>
            <div className="chk-card chk-g"><div className="chk-num">3</div><div className="chk-lbl">Безопасно</div><div className="chk-desc">Не нарушают действующие НПА</div></div>
          </div>
          <h3 style={{ fontSize: 14, fontWeight: 700, marginBottom: 12 }}>Иерархия законодательства Республики Беларусь</h3>
          <div className="hier-list">
            {HIERARCHY_DATA.map((h: any) => (
              <div key={h.n} className="hier-item">
                <span className="hi-num">{h.n}</span>
                <span className="hi-txt">{h.text}</span>
                <span className="hi-dot" style={{ background: h.ok ? 'var(--grn)' : 'var(--org)' }} />
              </div>
            ))}
          </div>
          <h3 style={{ fontSize: 14, fontWeight: 700, margin: '28px 0 12px' }}>Выявленные нарушения и противоречия</h3>
          <div className="viol-list">
            {VIOLATIONS_DATA.map((v: any) => (
              <div key={v.s} className={`viol-card${v.risk === 'orange' ? ' warn' : ''}`}>
                <div className="viol-top"><span className="viol-sec">{v.s}</span><span className={`bdg ${rBdg(v.risk)}`}>{rFull(v.risk)}</span></div>
                <p className="viol-txt">{v.text}</p>
                <p className="viol-law"><a href={v.href} target="_blank" rel="noopener">{v.law} · pravo.by ↗</a></p>
              </div>
            ))}
          </div>
          <div style={{ display: 'flex', gap: 10, marginTop: 22, flexWrap: 'wrap' }}>
            <button className="btn-dark" onClick={() => showToast('📄 Экспорт .docx')}>Скачать отчёт .docx</button>
            <button className="btn-outline" onClick={() => window.open('https://pravo.by', '_blank', 'noopener')}>Открыть на НЦПИ</button>
            <button className="btn-outline" onClick={() => showToast('✓ Ссылка скопирована')}>Поделиться</button>
          </div>
        </div>
      )}

      {activeChange && (
        <div className="modal-overlay" onClick={() => setActiveChange(null)}>
          <div className="modal modal-xl" onClick={e => e.stopPropagation()}>
            <div className="diff-blocks">
              <div className="diff-block">
                <div className="diff-lbl">Было</div>
                <div className="diff-txt">{activeChange.old}</div>
              </div>
              <div className="diff-block">
                <div className="diff-lbl">Стало</div>
                <div className="diff-txt">{activeChange.nw}</div>
              </div>
            </div>

            <div className="ch-meta">
              <div style={{ display: 'flex', gap: 40 }}>
                <div>
                  <div className="ch-meta-lbl">Раздел</div>
                  <div className="ch-meta-val">{activeChange.s}</div>
                </div>
                <div>
                  <div className="ch-meta-lbl">Закон</div>
                  <div className="ch-meta-val">{activeChange.law}</div>
                </div>
              </div>
              <div>
                <span className={`bdg ${rBdg(activeChange.risk)}`}>{rFull(activeChange.risk)}</span>
              </div>
            </div>

            <div className="ch-rec">
              <div className="ch-rec-lbl">Рекомендации</div>
              <div className="ch-rec-txt">{activeChange.rec}</div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// ─────────────────────────────────────────────────────────
// CHAIN PAGE
// ─────────────────────────────────────────────────────────
export function ChainPage() {
  const navigate = useNavigate()
  const showToast = useUIStore(s => s.showToast)

  const dotClass = (risk: Risk) => ({ red: 'ct-dot-r', orange: 'ct-dot-o', green: 'ct-dot-g' }[risk] || 'ct-dot-g')
  const segW = (v: typeof CHAIN_VERSIONS_DATA[0], key: 'red' | 'org' | 'grn') => {
    const w = (v.red + v.org + v.grn) || 1
    return Math.round(v[key] / w * 60)
  }

  const svgContent = useMemo(() => {
    const W = 400, H = 180, PAD = { t: 20, r: 20, b: 30, l: 30 }
    const cw = W - PAD.l - PAD.r, ch = H - PAD.t - PAD.b
    const data = [...CHAIN_VERSIONS_DATA].reverse()
    const maxY = Math.max(...data.map(d => d.red + d.org + d.grn)) || 10
    const xs = data.map((_, i) => PAD.l + (i / (data.length - 1)) * cw)
    const line = (key: 'red' | 'org' | 'grn') => data.map((d, i) => `${xs[i]},${PAD.t + ch - (d[key] / maxY) * ch}`).join(' ')
    const labels = data.map((d, i) => `<text x="${xs[i]}" y="${H - 5}" text-anchor="middle" font-size="10" fill="#a1a1aa">${d.ver}</text>`).join('')
    const gridY = [0, .25, .5, .75, 1].map(f => { const y = PAD.t + ch * (1 - f); return `<line x1="${PAD.l}" y1="${y}" x2="${W - PAD.r}" y2="${y}" stroke="#e4e4e7" stroke-width="1"/>`; }).join('')
    const dots = data.map((d, i) => `<circle cx="${xs[i]}" cy="${PAD.t + ch - (d.grn / maxY) * ch}" r="3.5" fill="#22c55e"/><circle cx="${xs[i]}" cy="${PAD.t + ch - (d.org / maxY) * ch}" r="3.5" fill="#f97316"/><circle cx="${xs[i]}" cy="${PAD.t + ch - (d.red / maxY) * ch}" r="3.5" fill="#ef4444"/>`).join('')
    return `${gridY}<polyline points="${line('grn')}" fill="none" stroke="#22c55e" stroke-width="2"/><polyline points="${line('org')}" fill="none" stroke="#f97316" stroke-width="2"/><polyline points="${line('red')}" fill="none" stroke="#ef4444" stroke-width="2"/>${dots}${labels}`
  }, [])

  return (
    <div className="page" style={{ padding: '0 20px 32px' }}>
      <div className="page-topnav" style={{ padding: '12px 0 0' }}>
        <button className="topnav-btn" onClick={() => navigate('/acts')}>
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><polyline points="15,18 9,12 15,6" /></svg>
          Назад к актам
        </button>
      </div>
      <div className="chain-header">
        <div><h1 className="chain-title">Цепочка редакций</h1><p className="chain-sub">Правила внутреннего трудового распорядка · ООО «ТехПром» · 5 версий</p></div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn-outline" onClick={() => showToast('📄 Экспорт истории')}>Экспорт истории</button>
          <button className="btn-dark" onClick={() => navigate('/compare/1')}>Сравнить любые две версии</button>
        </div>
      </div>
      <div className="chain-stats">
        {[{ n: '5', l: 'версий' }, { n: '14 мес', l: 'охвачено' }, { n: '6', l: 'нарушений', cls: 'cstat-r' }, { n: '9', l: 'на проверку', cls: 'cstat-o' }, { n: '12', l: 'безопасно', cls: 'cstat-g' }].map(s => (
          <div key={s.l} className={`cstat ${s.cls || ''}`}><span className="cstat-n">{s.n}</span><span className="cstat-l">{s.l}</span></div>
        ))}
      </div>
      <div className="chain-body">
        <div>
          <h3 className="chain-col-title">История версий</h3>
          <div className="chain-timeline">
            {CHAIN_VERSIONS_DATA.map(v => (
              <div key={v.ver} className="ct-item">
                <div className={`ct-dot ${dotClass(v.risk)}`} />
                <div className="ct-card">
                  <div className="ct-head"><span className="ct-ver">{v.ver}</span><span className="ct-date">{v.date}</span></div>
                  <div className="ct-title">{v.title}</div>
                  <div className="ct-meta">
                    <span className="ct-changes">{v.changes} изм.</span>
                    <div className="ct-bars">
                      {v.red > 0 && <span className="ct-seg bs-r" style={{ width: segW(v, 'red') + 'px' }} />}
                      {v.org > 0 && <span className="ct-seg bs-o" style={{ width: segW(v, 'org') + 'px' }} />}
                      {v.grn > 0 && <span className="ct-seg bs-g" style={{ width: segW(v, 'grn') + 'px' }} />}
                    </div>
                  </div>
                  <div className="ct-author">Автор: {v.author}</div>
                  <div className="ct-actions">
                    <button className="ct-btn" onClick={() => navigate('/compare/1')}>Сравнить</button>
                    <button className="ct-btn" onClick={() => showToast('📄 Экспорт')}>Отчёт</button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
        <div>
          <h3 className="chain-col-title">Динамика рисков по версиям</h3>
          <svg className="chain-svg" viewBox="0 0 400 180" xmlns="http://www.w3.org/2000/svg" dangerouslySetInnerHTML={{ __html: svgContent }} />
          <div className="chain-legend">
            <span><span className="rdot r" />Противоречия</span>
            <span><span className="rdot o" />Проверить</span>
            <span><span className="rdot g" />Безопасно</span>
          </div>
        </div>
      </div>
    </div>
  )
}

// ─────────────────────────────────────────────────────────
// ASSISTANT PAGE
// ─────────────────────────────────────────────────────────
export function AssistantPage() {
  const showToast = useUIStore(s => s.showToast)
  const { messages, input, setInput, started, send, newChat, msgsRef } = useAssistant()
  const [activeChatId, setActiveChatId] = useState(0)

  const chatHistory = [
    'Анализ ПИТР v2', 'Проверка ст.45 Закона', 'Сравнение НК РБ 2024', 'Декрет №4 изменения', 'ТК РБ ст.73 анализ'
  ]
  const suggestions = [
    'Объясни изменение в п.2.4', 'Какие риски в новой редакции?', 'Найди противоречия с ТК РБ',
    'Сформируй юридическое заключение', 'Проверь соответствие Конституции РБ', 'Проанализируй цепочку редакций',
  ]

  const handleKey = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() }
  }

  return (
    <div className="page" style={{ padding: 0, overflow: 'hidden' }}>
      <div className="ass-layout">
        <div className="ass-sidebar">
          <button className="btn-dark sm" onClick={newChat}>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
            Новый чат
          </button>
          <div className="ass-hist-section">Недавние</div>
          {chatHistory.map((ch, i) => (
            <button key={i} className={`ass-hist-item${activeChatId === i ? ' active' : ''}`} onClick={() => setActiveChatId(i)}>{ch}</button>
          ))}
        </div>
        <div className="ass-main">
          <div className="ass-msgs" ref={msgsRef}>
            {!started ? (
              <div className="ass-welcome">
                <div className="ass-wlc-icon">
                  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96-.46 2.5 2.5 0 0 1-2.96-3.08 3 3 0 0 1-.34-5.58 2.5 2.5 0 0 1 1.32-4.24 2.5 2.5 0 0 1 1.44-4.14" /><path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96-.46 2.5 2.5 0 0 0 2.96-3.08 3 3 0 0 0 .34-5.58 2.5 2.5 0 0 0-1.32-4.24 2.5 2.5 0 0 0-1.44-4.14" /></svg>
                </div>
                <h2 style={{ fontSize: 18, fontWeight: 700, marginBottom: 8 }}>legist Ассистент</h2>
                <p style={{ fontSize: '13.5px', color: 'var(--muted)', maxWidth: 420, lineHeight: 1.6 }}>
                  Задайте вопрос о загруженных актах, попросите объяснить изменение или проверить соответствие законодательству
                </p>
                <div className="suggestions">
                  {suggestions.map(s => <button key={s} className="sug" onClick={() => send(s)}>{s}</button>)}
                </div>
              </div>
            ) : (
              <div className="chat-msgs">
                {messages.map((m, i) => (
                  <div key={i} className={`cm ${m.role}`}>
                    <div className="cm-av">{m.role === 'user' ? 'Вы' : 'AI'}</div>
                    <div className="cm-bub">
                      {m.typing ? <div className="typing"><span /><span /><span /></div> : <span dangerouslySetInnerHTML={{ __html: m.html }} />}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
          <div className="ass-input-bar">
            <div className="ass-input-meta">
              <button className="ass-chip" onClick={() => showToast('Откройте нужный акт и вернитесь в ассистент')}>
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="3" y="3" width="18" height="18" rx="2" /></svg>
                Выбрать проект
              </button>
              <button className="ass-chip">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="12" cy="12" r="10" /><line x1="2" y1="12" x2="22" y2="12" /><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" /></svg>
                Все ресурсы
              </button>
            </div>
            <div className="ass-input-box">
              <textarea className="ass-ta" rows={1} placeholder="Спросите, найдите или создайте что-нибудь..."
                value={input}
                onChange={e => {
                  setInput(e.target.value)
                  e.target.style.height = 'auto'
                  e.target.style.height = Math.min(e.target.scrollHeight, 130) + 'px'
                }}
                onKeyDown={handleKey} />
              <button className="ass-send" onClick={() => send()}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><line x1="12" y1="5" x2="12" y2="19" /><polyline points="19,12 12,5 5,12" /></svg>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}