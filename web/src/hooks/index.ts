import { useState, useCallback, useRef } from 'react'
import DOMPurify from 'dompurify'
import { useUIStore } from '../store'
import { PROGRESS_STEP_LABELS, PROGRESS_STEPS } from '../utils/helpers'
import { apiFetch } from '../utils/api'
import type { FileInfo } from '../types'

// ── useToast ──────────────────────────────────────────────
export function useToast() {
  return useUIStore((s) => s.showToast)
}

// ── useFileUpload ─────────────────────────────────────────
export function useFileUpload() {
  const [oldFile, setOldFile] = useState<FileInfo | null>(null)
  const [newFile, setNewFile] = useState<FileInfo | null>(null)
  // ✅ Раздельные флаги для каждого файла вместо одного общего
  const [loadingOld, setLoadingOld] = useState(false)
  const [loadingNew, setLoadingNew] = useState(false)
  const [dzOldOver, setDzOldOver] = useState(false)
  const [dzNewOver, setDzNewOver] = useState(false)
  const showToast = useToast()

  const uploadFile = useCallback(async (file: File, side: 'old' | 'new') => {
    const token = localStorage.getItem('legist_token')
    if (!token) {
      showToast('⚠ Авторизуйтесь для загрузки файлов')
      return
    }

    // ✅ Проверка максимального размера файла
    const MAX_FILE_SIZE = 100 * 1024 * 1024 // 100 MB
    if (file.size > MAX_FILE_SIZE) {
      showToast(`⚠ Макс размер файла 100 МБ. Ваш файл ${(file.size / 1024 / 1024).toFixed(1)} МБ`)
      return
    }

    // ✅ Используй правильный флаг для каждого файла
    if (side === 'old') setLoadingOld(true)
    else setLoadingNew(true)

    // Функция для одной попытки загрузки (вспомогательная)
    const attemptUpload = async (attemptNum: number = 0): Promise<any> => {
      const formData = new FormData()

      // Создаём уникальное имя файла: name-timestamp-random.ext
      // Гарантирует уникальность даже при одновременной загрузке нескольких файлов
      const ext = file.name.split('.').pop() || ''
      const nameWithoutExt = file.name.replace(/\.[^/.]+$/, '')
      const timestamp = Date.now()
      // Используем более сильную случайность: микросекунды + криптографический UUID
      const random = crypto.getRandomValues(new Uint8Array(6)).reduce((a, b) => a + b.toString(16).padStart(2, '0'), '')

      // На повторных попытках добавляем суффикс к основному UUID
      const uniqueName = attemptNum > 0
        ? `${nameWithoutExt}-${timestamp}-${random}-retry${attemptNum}.${ext}`
        : `${nameWithoutExt}-${timestamp}-${random}.${ext}`

      // Создаём новый File с уникальным именем
      const uniqueFile = new File([file], uniqueName, { type: file.type })
      formData.append('file', uniqueFile)

      try {
        const res = await apiFetch('/api/files', {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${token}`,
            'Idempotency-Key': crypto.randomUUID(),
          },
          body: formData,
        })

        const data = await res.json()
        console.log('Upload response:', { status: res.status, data, filename: uniqueName })

        // Если 409 Conflict (файл уже существует), попробуем ещё раз с новым именем
        if (res.status === 409 && attemptNum < 2) {
          console.warn(`Upload conflict (409), retrying with attempt ${attemptNum + 1}...`, data)
          // Ждём немного перед повторной попыткой (экспоненциальная задержка)
          await new Promise(r => setTimeout(r, 200 * (attemptNum + 1)))
          return attemptUpload(attemptNum + 1)
        }

        if (!res.ok) {
          console.error('Upload error response:', { status: res.status, data, filename: uniqueName })
          throw new Error(data.error?.message || `Ошибка загрузки (${res.status})`)
        }

        return data
      } catch (err) {
        if (attemptNum < 2 && err instanceof Error && err.message.includes('409')) {
          console.warn(`Upload failed with 409, retrying attempt ${attemptNum + 1}...`)
          await new Promise(r => setTimeout(r, 200 * (attemptNum + 1)))
          return attemptUpload(attemptNum + 1)
        }
        throw err
      }
    }

    try {
      const data = await attemptUpload()

      const info: FileInfo = { id: data.id, name: data.filename, size: data.size || file.size }
      if (side === 'old') setOldFile(info)
      else setNewFile(info)

      showToast(`✓ Файл ${data.filename} загружен`)
      return data.id
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Ошибка загрузки'
      console.error('File upload error:', err)

      // Более понятная ошибка для пользователя
      let userFriendlyMsg = msg
      if (msg.includes('already exists')) {
        userFriendlyMsg = 'Документ с такими данными уже загружен. Загрузите другую версию документа или другой документ.'
      }

      showToast('⚠ ' + userFriendlyMsg)
    } finally {
      // ✅ Отключаем правильный флаг для каждого файла
      if (side === 'old') setLoadingOld(false)
      else setLoadingNew(false)
    }
  }, [showToast])

  const onDrop = useCallback((e: React.DragEvent, side: 'old' | 'new') => {
    e.preventDefault()
    setDzOldOver(false)
    setDzNewOver(false)
    const file = e.dataTransfer.files[0]
    if (file) uploadFile(file, side)
  }, [uploadFile])

  const onSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>, side: 'old' | 'new') => {
    const file = e.target.files?.[0]
    if (file) uploadFile(file, side)
  }, [uploadFile])

  return {
    oldFile, newFile, dzOldOver, dzNewOver,
    // ✅ Возвращаем оба флага для раздельного управления UI
    loadingOld, loadingNew,
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
      try {
        const data = JSON.parse(e.data)
        setPct(data.percentage || 50)
        setLabel(data.message || 'Обработка...')
      } catch (err) {
        console.error('Failed to parse SSE progress:', err)
      }
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

  // ✅ Безопасное экранирование HTML с защитой от XSS
  const sanitizeHtml = (s: string) => {
    // Сначала экранируем базовые символы
    const escaped = s
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/\n/g, '<br>')

    // Потом применяем форматирование **bold**
    const formatted = escaped.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')

    // В конце очищаем через DOMPurify для максимальной безопасности
    return DOMPurify.sanitize(formatted, {
      ALLOWED_TAGS: ['br', 'strong'],
      ALLOWED_ATTR: []
    })
  }

  const send = useCallback(async (text?: string) => {
    const msg = (text ?? input).trim()
    if (!msg) return
    setInput(''); setStarted(true)
    
    setMessages(prev => [...prev, { role: 'user', html: sanitizeHtml(msg) }])
    const aiMsgIdx = messages.length + 1
    setMessages(prev => [...prev, { role: 'ai', html: '', typing: true }])

    const token = localStorage.getItem('legist_token')
    
    try {
      const res = await apiFetch('/api/chat', {
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
        i === aiMsgIdx ? { role: 'ai', html: sanitizeHtml(reply), typing: false } : m
      ))
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Неизвестная ошибка'
      setMessages(prev => prev.map((m, i) =>
        i === aiMsgIdx ? { role: 'ai', html: sanitizeHtml('⚠ ' + msg), typing: false } : m
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
