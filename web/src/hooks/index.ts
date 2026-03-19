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
  const [dzOldOver, setDzOldOver] = useState(false)
  const [dzNewOver, setDzNewOver] = useState(false)
  const showToast = useToast()

  const handleFile = useCallback((file: File, side: 'old' | 'new') => {
    const ext = file.name.split('.').pop()?.toLowerCase()
    if (!['pdf', 'docx'].includes(ext || '')) {
      showToast('⚠ Только .pdf и .docx файлы')
      return
    }
    const info: FileInfo = { name: file.name, size: file.size }
    if (side === 'old') setOldFile(info)
    else setNewFile(info)
  }, [showToast])

  const onDrop = useCallback((e: React.DragEvent, side: 'old' | 'new') => {
    e.preventDefault()
    setDzOldOver(false); setDzNewOver(false)
    const file = e.dataTransfer.files[0]
    if (file) handleFile(file, side)
  }, [handleFile])

  const onSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>, side: 'old' | 'new') => {
    const file = e.target.files?.[0]
    if (file) handleFile(file, side)
  }, [handleFile])

  return {
    oldFile, newFile, dzOldOver, dzNewOver,
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
  const ivRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const start = useCallback((onDone: () => void) => {
    setRunning(true); setPct(0); setStep(0)
    setLabel(PROGRESS_STEP_LABELS[0])
    const stepAt = [20, 40, 60, 80, 100]
    let curPct = 0, si = 0

    ivRef.current = setInterval(() => {
      curPct = Math.min(curPct + 4, 100)
      setPct(curPct)
      const ns = stepAt.findIndex(s => curPct <= s)
      if (ns !== si && ns >= 0) {
        si = ns
        setStep(si)
        setLabel(PROGRESS_STEP_LABELS[si] || '')
      }
      if (curPct >= 100) {
        clearInterval(ivRef.current!)
        setTimeout(() => { setRunning(false); onDone() }, 400)
      }
    }, 50)
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

  const send = useCallback((text?: string) => {
    const msg = (text ?? input).trim()
    if (!msg) return
    setInput(''); setStarted(true)
    setMessages(prev => [...prev, { role: 'user', html: esc(msg) }])
    const idx = messages.length + 1
    setMessages(prev => [...prev, { role: 'ai', html: '', typing: true }])
    setTimeout(() => {
      setMessages(prev => prev.map((m, i) =>
        i === idx ? { role: 'ai', html: esc(getMockReply(msg)) } : m
      ))
      setTimeout(() => {
        if (msgsRef.current) msgsRef.current.scrollTop = msgsRef.current.scrollHeight
      }, 0)
    }, 900 + Math.random() * 500)
  }, [input, messages.length])

  const newChat = useCallback(() => { setMessages([]); setStarted(false) }, [])

  return { messages, input, setInput, started, send, newChat, msgsRef }
}
