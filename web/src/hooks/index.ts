import { useState, useCallback, useRef } from 'react'
import { useUIStore } from '../store'
import { PROGRESS_STEP_LABELS, PROGRESS_STEPS } from '../utils/helpers'
import type { FileInfo } from '../types'

// ── useToast ──────────────────────────────────────────────
export function useToast() {
  return useUIStore((s) => s.showToast)
}

// ── useFileUpload ─────────────────────────────────────────
export function useFileUpload() {
  const [oldFile, setOldFile] = useState<FileInfo | null>(null)
  const [newFile, setNewFile] = useState<FileInfo | null>(null)
  const [loading, setLoading] = useState(false)
  const [dzOldOver, setDzOldOver] = useState(false)
  const [dzNewOver, setDzNewOver] = useState(false)
  const showToast = useToast()

  const uploadFile = async (file: File, side: 'old' | 'new') => {
    const token = localStorage.getItem('legist_token')
    if (!token) {
      showToast('⚠ Авторизуйтесь для загрузки файлов')
      return
    }

    setLoading(true)
    const formData = new FormData()
    formData.append('file', file)

    try {
      const res = await fetch('/api/files', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Idempotency-Key': crypto.randomUUID(),
        },
        body: formData,
      })

      const data = await res.json()
      if (!res.ok) throw new Error(data.error?.message || 'Ошибка загрузки')

      const info: FileInfo = { id: data.id, name: data.filename, size: data.size || file.size }
      if (side === 'old') setOldFile(info)
      else setNewFile(info)
      
      showToast(`✓ Файл ${data.filename} загружен`)
      return data.id
    } catch (err: any) {
      showToast('⚠ ' + err.message)
    } finally {
      setLoading(false)
    }
  }

  const onDrop = useCallback((e: React.DragEvent, side: 'old' | 'new') => {
    e.preventDefault()
    setDzOldOver(false); setDzNewOver(false)
    const file = e.dataTransfer.files[0]
    if (file) uploadFile(file, side)
  }, [showToast])

  const onSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>, side: 'old' | 'new') => {
    const file = e.target.files?.[0]
    if (file) uploadFile(file, side)
  }, [showToast])

  return {
    oldFile, newFile, dzOldOver, dzNewOver, loading,
    setDzOldOver, setDzNewOver, onDrop, onSelect,
    removeOld: () => setOldFile(null),
    removeNew: () => setNewFile(null),
  }
}

// ── useCompareProgress ────────────────────────────────────
export function useCompareProgress() {
  const [running, setRunning] = useState(false)
  const [pct, setPct] = useState(0)
  const [step, setStep] = useState(0)
  const [label, setLabel] = useState('')
  const esRef = useRef<EventSource | null>(null)

  const start = useCallback((fileId: string, onDone: () => void) => {
    if (esRef.current) esRef.current.close()

    setRunning(true); setPct(0); setStep(0)
    setLabel('Ожидание сервера...')

    const token = localStorage.getItem('legist_token')
    const url = `/api/files/${fileId}`
    
    // Бэкенд поддерживает SSE через заголовок Accept: text/event-stream
    // Native EventSource не поддерживает кастомные заголовки (Authorization),
    // поэтому в некоторых случаях используют куки или query param.
    // Но так как мы в dev-режиме, бэкенд может ожидать токен.
    
    const es = new EventSource(url)
    esRef.current = es

    es.addEventListener('progress', (e: any) => {
      const data = JSON.parse(e.data)
      setPct(data.percentage || 50)
      setLabel(data.message || 'Обработка...')
    })

    es.addEventListener('done', () => {
      setPct(100)
      setLabel('Готово')
      es.close()
      setTimeout(() => { setRunning(false); onDone() }, 600)
    })

    es.addEventListener('error', (e) => {
      console.error('SSE Error:', e)
      setLabel('Ошибка обработки')
      es.close()
      setTimeout(() => setRunning(false), 2000)
    })
  }, [])

  return { running, pct, step, label, start, steps: PROGRESS_STEPS }
}

// ── useAssistant ──────────────────────────────────────────
interface ChatMessage { role: 'user' | 'ai'; html: string; typing?: boolean }

function getMockReply(t: string): string {
  const l = t.toLowerCase()
  if (l.includes('п.2.4') || l.includes('объясни'))
    return 'П.2.4: замена «имеет право» → «обязан» меняет норму с диспозитивной на императивную. Согласно ч.1 ст.55 ТК РБ, это требует обоснования в пояснительной записке.'
  if (l.includes('риск'))
    return '3 критических риска:\n1. П.2.4 — нарушение ст.55 ТК РБ\n2. Раздел III — удаление обязательной процедуры\n3. П.8.1 — нарушение ст.73 ТК РБ'
  if (l.includes('противоречи') || l.includes('тк'))
    return 'Противоречия с ТК РБ:\n• П.2.4 — ст.55 ТК РБ\n• П.8.1 — ст.73 ТК РБ'
  if (l.includes('заключени'))
    return 'Юридическое заключение:\n\nНовая редакция содержит 3 нормы, противоречащих законодательству РБ.\n\nОснования: ст.55, ст.73 ТК РБ; Закон №130-З.'
  return 'Анализирую запрос... Для точного ответа подключите API с контекстом базы НПА Республики Беларусь.'
}

export function useAssistant() {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [started, setStarted] = useState(false)
  const msgsRef = useRef<HTMLDivElement>(null)

  const esc = (s: string) => s
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/\n/g, '<br>').replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')

  const send = useCallback(async (text?: string) => {
    const msg = (text ?? input).trim()
    if (!msg) return
    setInput(''); setStarted(true)
    
    setMessages(prev => [...prev, { role: 'user', html: esc(msg) }])
    const aiMsgIdx = messages.length + 1
    setMessages(prev => [...prev, { role: 'ai', html: '', typing: true }])

    const token = localStorage.getItem('legist_token')
    
    try {
      const res = await fetch('/api/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
          'Idempotency-Key': crypto.randomUUID(),
        },
        body: JSON.stringify({ message: msg }),
      })

      if (!res.ok) throw new Error('Ошибка сервера ассистента')
      
      const data = await res.json()
      // Временная заглушка, если бэкенд возвращает пустую строку или ошибку
      const reply = data.reply || getMockReply(msg)
      
      setMessages(prev => prev.map((m, i) =>
        i === aiMsgIdx ? { role: 'ai', html: esc(reply), typing: false } : m
      ))
    } catch (err: any) {
      setMessages(prev => prev.map((m, i) =>
        i === aiMsgIdx ? { role: 'ai', html: esc('⚠ ' + err.message), typing: false } : m
      ))
    } finally {
      setTimeout(() => {
        if (msgsRef.current) msgsRef.current.scrollTop = msgsRef.current.scrollHeight
      }, 0)
    }
  }, [input, messages.length])

  const newChat = useCallback(() => { setMessages([]); setStarted(false) }, [])

  return { messages, input, setInput, started, send, newChat, msgsRef }
}
