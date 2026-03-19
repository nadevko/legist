import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useUIStore } from '../store'

export function useKeyboard() {
  const navigate = useNavigate()
  const { setModalShortcuts, setModalNtsi, showToast } = useUIStore()
  const kbSeq = useRef('')
  const kbTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const tag = (document.activeElement?.tagName || '').toLowerCase()
      if (['input', 'textarea', 'select'].includes(tag)) return

      // Esc — close modals
      if (e.key === 'Escape') {
        useUIStore.setState({ modalDel: false, modalNtsi: false, modalShortcuts: false })
        return
      }

      // Ctrl shortcuts
      if (e.ctrlKey || e.metaKey) {
        if (e.key === 'k') { e.preventDefault(); setModalNtsi(true) }
        if (e.key === 'e') { e.preventDefault(); showToast('📄 Экспорт .docx (требуется backend)') }
        return
      }

      // Single keys
      switch (e.key) {
        case '?': setModalShortcuts(true); return
        case 'n': case 'N': navigate('/'); return
        case 'v': case 'V': return // handled per-page
      }

      // Two-key sequences: G + A/H/I/C/U
      if (e.key === 'g' || e.key === 'G') {
        kbSeq.current = 'g'
        if (kbTimer.current) clearTimeout(kbTimer.current)
        kbTimer.current = setTimeout(() => { kbSeq.current = '' }, 1000)
        return
      }
      if (kbSeq.current === 'g') {
        kbSeq.current = ''
        if (kbTimer.current) clearTimeout(kbTimer.current)
        const k = e.key.toLowerCase()
        if (k === 'a') navigate('/acts')
        if (k === 'h') navigate('/')
        if (k === 'i') navigate('/assistant')
        if (k === 'c') navigate('/chain/1')
        if (k === 'u') navigate('/')
      }
    }

    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [navigate, setModalShortcuts, setModalNtsi, showToast])
}
