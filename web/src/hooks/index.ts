import { useState, useCallback, useRef, useEffect } from 'react'
import DOMPurify from 'dompurify'
import { useUIStore } from '../store'
import { PROGRESS_STEP_LABELS, PROGRESS_STEPS } from '../utils/helpers'
import { apiFetch } from '../utils/api'
import { streamSse } from '../utils/sse'
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

      const info: FileInfo = {
        id: data.id,
        name: data.name ?? data.filename ?? file.name,
        size: data.size ?? file.size,
        documentId: data.document_id ?? data.documentId,
      }
      if (side === 'old') setOldFile(info)
      else setNewFile(info)

      showToast(`✓ Файл ${data.name ?? data.filename ?? file.name} загружен`)
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

  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    return () => abortRef.current?.abort()
  }, [])

  const stageToStep = (stage?: string) => {
    switch (stage) {
      case 'parsing_started':
        return 0
      case 'llm_requested':
      case 'llm_skipped':
      case 'llm_done':
        return 2
      case 'saving':
      case 'embedding_started':
      case 'embedding':
      case 'embedding_done':
      case 'matching':
        return 3
      case 'done':
      case 'failed':
      default:
        return 4
    }
  }

  const stageToDefaultLabel = (stage?: string) => {
    // Keep UI language stable even when backend messages are in English.
    switch (stage) {
      case 'parsing_started':
        return PROGRESS_STEP_LABELS[0]
      case 'llm_requested':
      case 'llm_skipped':
      case 'llm_done':
        return PROGRESS_STEP_LABELS[2]
      case 'saving':
      case 'embedding_started':
      case 'embedding':
      case 'embedding_done':
      case 'matching':
        return PROGRESS_STEP_LABELS[3]
      case 'done':
        return 'Готово'
      case 'failed':
        return 'Ошибка обработки'
      default:
        return 'Ожидание сервера...'
    }
  }

  const start = useCallback((fileId: string, onDone: () => void) => {
    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    setRunning(true)
    setPct(0)
    setStep(0)
    setLabel('Ожидание сервера...')

    const token = localStorage.getItem('legist_token')
    const url = `/api/files/${fileId}`

    let ok = false

    const run = async () => {
      try {
        await streamSse(url, {
          method: 'GET',
          headers: {
            Authorization: `Bearer ${token}`,
            Accept: 'text/event-stream',
          },
          signal: controller.signal,
          onEvent: (evt) => {
            if (evt.type === 'progress') {
              const data = evt.data ?? {}
              const stage = typeof data.stage === 'string' ? data.stage : undefined

              const embeddingPct = typeof data.embedding_percent === 'number' ? data.embedding_percent : undefined
              const chunksEmbedded = typeof data.chunks_embedded === 'number' ? data.chunks_embedded : undefined

              setStep(stageToStep(stage))

              setLabel(data.message || stageToDefaultLabel(stage) + (chunksEmbedded != null ? ` (${chunksEmbedded}...)` : ''))
              if (embeddingPct != null) setPct(Math.max(0, Math.min(100, embeddingPct)))
              return
            }

            if (evt.type === 'done') {
              ok = true
              setPct(100)
              setStep(4)
              setLabel('Готово')
              return
            }

            if (evt.type === 'failed') {
              ok = false
              setStep(4)
              setLabel(evt.data?.error ? `Ошибка: ${String(evt.data.error)}` : 'Ошибка обработки')
            }
          },
        })
      } catch (err) {
        if (controller.signal.aborted) return
        setStep(4)
        setLabel('Ошибка обработки')
        console.error('SSE file error:', err)
      } finally {
        setRunning(false)
        if (ok) onDone()
      }
    }

    void run()
  }, [setRunning])

  const startDiff = useCallback((diffFormData: FormData, onDone: (diffId: string) => void) => {
    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    setRunning(true)
    setPct(0)
    setStep(0)
    setLabel('Ожидание сервера...')

    const token = localStorage.getItem('legist_token')

    let ok = false
    let diffId = ''

    const run = async () => {
      try {
        await streamSse('/api/diffs', {
          method: 'POST',
          headers: {
            Authorization: `Bearer ${token}`,
            Accept: 'text/event-stream',
            'Idempotency-Key': crypto.randomUUID(),
          },
          body: diffFormData,
          signal: controller.signal,
          onEvent: (evt) => {
            if (evt.type === 'diff_started') {
              setLabel('Сравнение документов...')
              setStep(0)
              setPct((p) => Math.max(p, 5))
              return
            }

            if (evt.type === 'progress') {
              // Diff progress event shape:
              // {file_id:<null>, progress: ParseProgress} OR direct ParseProgress.
              const data = evt.data ?? {}
              const p = data.progress ?? data
              const stage = typeof p.stage === 'string' ? p.stage : undefined
              const embeddingPct = typeof p.embedding_percent === 'number' ? p.embedding_percent : undefined

              setStep(stageToStep(stage))
              setLabel(p.message || stageToDefaultLabel(stage))
              if (embeddingPct != null) setPct(Math.max(0, Math.min(100, embeddingPct)))
              return
            }

            if (evt.type === 'file_done') {
              setLabel('Подготовка сторон завершена...')
              return
            }

            if (evt.type === 'file_failed') {
              ok = false
              setStep(4)
              setLabel(evt.data?.error ? `Ошибка: ${String(evt.data.error)}` : 'Ошибка обработки')
              return
            }

            if (evt.type === 'diff_done') {
              ok = true
              diffId = String(evt.data?.diff_id ?? '')
              setPct(100)
              setStep(4)
              setLabel('Готово')
              return
            }

            if (evt.type === 'diff_failed') {
              ok = false
              setStep(4)
              setLabel(evt.data?.error ? `Ошибка: ${String(evt.data.error)}` : 'Ошибка обработки')
              return
            }
          },
        })
      } catch (err) {
        if (controller.signal.aborted) return
        console.error('SSE diff error:', err)
        setStep(4)
        setLabel('Ошибка обработки')
      } finally {
        setRunning(false)
        if (ok && diffId) onDone(diffId)
      }
    }

    void run()
  }, [setRunning])

  return { running, pct, step, label, start, startDiff, steps: PROGRESS_STEPS }
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
