const API_URL = import.meta.env.VITE_API_URL || 'https://legist-api-dev.up.railway.app'

export const getApiUrl = (path: string) => {
  if (path.startsWith('http')) return path
  return `${API_URL}${path.startsWith('/api') ? path : `/api${path}`}`
}

// ✅ Добавлен Network Timeout (30 секунд)
export const apiFetch = async (path: string, options?: RequestInit, timeout = 30000) => {
  const url = getApiUrl(path)
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), timeout)

  try {
    const res = await fetch(url, { ...options, signal: controller.signal })
    clearTimeout(timeoutId)
    return res
  } catch (err) {
    clearTimeout(timeoutId)
    // Если ошибка из-за timeout, показываем понятное сообщение
    if (err instanceof Error && err.name === 'AbortError') {
      throw new Error('Запрос заняло слишком много времени (30 сек). Проверьте интернет-соединение.')
    }
    throw err
  }
}