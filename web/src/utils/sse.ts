import { getApiUrl } from './api'

// Backend uses a Stripe-like SSE format:
// - event: message
// - data: {"type":"...", "data":{...}}
// - empty line terminates one event frame
export type StripeLikeSseEvent = {
  type: string
  data: any
}

export type StreamSseOptions = {
  method?: string
  headers?: Record<string, string>
  body?: BodyInit | null
  signal?: AbortSignal
  onEvent: (evt: StripeLikeSseEvent) => void
  onError?: (err: unknown) => void
}

function parseSseFrame(frame: string): StripeLikeSseEvent | null {
  const lines = frame.split(/\r?\n/)
  let dataLines: string[] = []

  for (const rawLine of lines) {
    const line = rawLine.trim()
    if (!line) continue
    if (line.startsWith('data:')) {
      dataLines.push(line.slice('data:'.length).trim())
    }
  }

  if (dataLines.length === 0) return null

  const dataStr = dataLines.join('\n')
  if (!dataStr) return null

  try {
    const parsed = JSON.parse(dataStr)
    if (parsed && typeof parsed.type === 'string' && 'data' in parsed) {
      return { type: parsed.type, data: parsed.data }
    }
    // Fallback: sometimes the payload may not include {type,data}.
    if (parsed && typeof parsed === 'object') {
      return { type: String((parsed as any).type ?? 'message'), data: (parsed as any).data ?? parsed }
    }
  } catch {
    // Ignore malformed frames (e.g. keep-alives).
  }

  return null
}

// Streams SSE frames and calls onEvent for each parsed payload.
// Resolves when the connection ends, rejects on non-2xx responses.
export async function streamSse(urlOrPath: string, options: StreamSseOptions): Promise<void> {
  const url = getApiUrl(urlOrPath)

  const res = await fetch(url, {
    method: options.method ?? 'GET',
    headers: options.headers,
    body: options.body ?? undefined,
    signal: options.signal,
  })

  if (!res.ok) {
    let msg = `SSE request failed: ${res.status}`
    try {
      msg += ` ${await res.text()}`
    } catch {
      // ignore
    }
    throw new Error(msg)
  }

  if (!res.body) {
    throw new Error('SSE response has no body')
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder('utf-8')

  let buffer = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })

    // SSE frames end with a blank line.
    // We parse by splitting on the first "\n\n" occurrences.
    while (true) {
      const endIdx = buffer.indexOf('\n\n')
      if (endIdx === -1) break

      const frame = buffer.slice(0, endIdx)
      buffer = buffer.slice(endIdx + 2)

      const evt = parseSseFrame(frame)
      if (evt) options.onEvent(evt)
    }
  }

  // Best-effort parse of remaining buffered frame (if server ends without \n\n).
  const evt = parseSseFrame(buffer.trim())
  if (evt) options.onEvent(evt)

  options.onError?.(undefined)
}

