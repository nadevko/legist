# 🐛 AUDIT & ROADMAP - Legist Frontend

Полный анализ проекта: найденные баги, недоделки и что нужно сделать.

---

## 📊 СТАТУС ПРОЕКТА

**Готово:** 75% ✅
- ✅ Frontend интегрирован с Backend API
- ✅ Система загрузки файлов с retry логикой
- ✅ Авторизация (логин/регистрация)
- ✅ Основные страницы реализованы
- ✅ SSE прогресс обработки файлов
- ✅ Чат с AI ассистентом
- ✅ Глобальное состояние (Zustand)

**Нужно доделать:** 25% 🚧

---

## 🔴 КРИТИЧЕСКИЕ БАГИ (нужна срочная доработка)

### 1. **SSE без Authorization Header** ⚠️ ВАЖНО
**Файл:** `/src/hooks/index.ts` (line 156)

**Проблема:**
```typescript
const es = new EventSource(url)  // ← НЕТ Authorization!
```

EventSource встроенный в браузер **не поддерживает кастомные headers**, включая Authorization. Если бэкенд требует токен для доступа к SSE, это не будет работать на защищённых файлах других пользователей.

**Решение - Вариант 1 (простой):**
Если бэкенд поддерживает query параметр для токена:
```typescript
const token = localStorage.getItem('legist_token')
const url = `/api/files/${fileId}?token=${token}`
const es = new EventSource(url)
```

**Решение - Вариант 2 (правильный):**
Использовать fetch с ReadableStream вместо EventSource (более сложно, но правильно):
```typescript
const token = localStorage.getItem('legist_token')
const response = await fetch(`/api/files/${fileId}`, {
  headers: { 'Authorization': `Bearer ${token}` }
})
const reader = response.body?.getReader()
// ... парсить stream вручную
```

**Статус:** 🚨 Нужно спросить у бэкендера как он ожидает авторизацию для SSE

---

### 2. **Parallel File Upload Loading State Bug** ⚠️ ВАЖНО
**Файл:** `/src/hooks/index.ts` (line 16)

**Проблема:**
```typescript
const [loading, setLoading] = useState(false)
```

Это один флаг для обоих файлов (oldFile и newFile). Если пользователь загружает оба файла одновременно:
- Загружаешь файл 1 → loading = true
- Начинаешь загружать файл 2 → loading тоже true
- Файл 1 закончился → setLoading(false)
- Но файл 2 всё ещё загружается и loading неправильно = false

Результат: UI показывает что файл 2 загружен, хотя он всё ещё загружается.

**Решение:**
```typescript
// Раздельные флаги для каждого файла
const [loadingOld, setLoadingOld] = useState(false)
const [loadingNew, setLoadingNew] = useState(false)

// В uploadFile:
if (side === 'old') setLoadingOld(true)
else setLoadingNew(true)

// Вернуть оба флага
return { oldFile, newFile, loadingOld, loadingNew, ... }
```

**Статус:** 🔴 HIGH PRIORITY - может привести к потере данных

---

### 3. **POST /api/chat не в Swagger** ⚠️ ВАЖНО
**Файл:** `/src/hooks/index.ts` (line 182)

**Проблема:**
Endpoint `/api/chat` используется в useAssistant, но его нет в Swagger документации backend'а. Это значит:
- Endpoint может быть неправильным
- Структура body/response может быть неправильной
- Endpoint может быть незащищённым

**Используется:**
```typescript
const res = await apiFetch('/api/chat', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`,
  },
  body: JSON.stringify({ message: msg }),
})
```

**Статус:** 🟡 Нужно подтвердить у бэкендера что это правильный endpoint

---

### 4. **Нет обработки Refresh Token** ⚠️ ВАЖНО
**Файл:** `/src/store/index.ts` (line 50-53)

**Проблема:**
Когда accessToken истекает (expires_in обычно 1 час), приложение не может автоматически обновить токен используя refreshToken. Пользователь просто будет выкинут без объяснения.

**Используется:**
```typescript
localStorage.setItem('legist_token', data.access_token)
if (data.refresh_token) {
  localStorage.setItem('legist_refresh_token', data.refresh_token)
}
```

Но refreshToken никогда не используется!

**Решение:**
Добавить перехват 401 ошибок в apiFetch:
```typescript
export const apiFetch = async (path: string, options?: RequestInit) => {
  let res = await fetch(url, options)

  // Если 401 Unauthorized
  if (res.status === 401) {
    const refreshToken = localStorage.getItem('legist_refresh_token')
    if (refreshToken) {
      // Обновить токен
      const refreshRes = await fetch(getApiUrl('/api/tokens/refresh'), {
        method: 'POST',
        body: JSON.stringify({ refresh_token: refreshToken })
      })
      const refreshData = await refreshRes.json()
      localStorage.setItem('legist_token', refreshData.access_token)

      // Повторить оригинальный запрос с новым токеном
      options!.headers = {
        ...options?.headers,
        'Authorization': `Bearer ${refreshData.access_token}`
      }
      res = await fetch(url, options)
    } else {
      // Нет refreshToken, нужно логинитьсь заново
      useAuthStore.getState().logout()
      window.location.href = '/auth'
    }
  }
  return res
}
```

**Статус:** 🔴 HIGH PRIORITY

---

## 🟡 СЕРЬЁЗНЫЕ ПРОБЛЕМЫ (нужна доработка)

### 5. **Нет Network Timeout в apiFetch**
**Файл:** `/src/utils/api.ts`

**Проблема:**
Если сервер не отвечает, запрос будет ждать бесконечно (по умолчанию fetch не имеет timeout).

**Решение:**
```typescript
export const apiFetch = async (path: string, options?: RequestInit, timeout = 30000) => {
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), timeout)

  try {
    const res = await fetch(url, { ...options, signal: controller.signal })
    clearTimeout(timeoutId)
    return res
  } catch (err) {
    clearTimeout(timeoutId)
    if (err instanceof Error && err.name === 'AbortError') {
      throw new Error('Запрос заняло слишком много времени')
    }
    throw err
  }
}
```

**Статус:** 🟡 MEDIUM PRIORITY

---

### 6. **XSS Vulnerability в Chat** ⚠️ SECURITY ISSUE
**Файл:** `/src/hooks/index.ts` (line 166-168)

**Проблема:**
```typescript
const esc = (s: string) => s
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  .replace(/\n/g, '<br>').replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')

// ... потом в setMessages:
const reply = data.reply || getMockReply(msg)
setMessages(prev => [...prev, { role: 'ai', html: esc(reply), typing: false }])

// И в компоненте:
<div dangerouslySetInnerHTML={{ __html: m.html }} />
```

Если `data.reply` содержит вредоносный HTML, даже после `esc()` функция не защищает от атак типа:
```html
<img src=x onerror="alert('XSS')">
```

**Решение:**
```typescript
// Использовать React.createElement вместо dangerouslySetInnerHTML
<div>{m.html}</div>  // просто текст как текст

// Или использовать проверенную библиотеку:
import DOMPurify from 'dompurify'
<div dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(m.html) }} />
```

**Статус:** 🔴 SECURITY - нужно срочно исправить

---

### 7. **Нет обработки больших файлов**
**Файл:** `/src/hooks/index.ts` (line 21-111)

**Проблема:**
Нет проверки размера файла перед загрузкой. Если пользователь будет загружать файл 1GB, браузер едва ли сможет это обработать.

**Решение:**
```typescript
const MAX_FILE_SIZE = 50 * 1024 * 1024; // 50 MB

const uploadFile = useCallback(async (file: File, side: 'old' | 'new') => {
  if (file.size > MAX_FILE_SIZE) {
    showToast(`⚠ Макс размер файла ${MAX_FILE_SIZE / 1024 / 1024}MB`)
    return
  }
  // ...
}, [showToast])
```

**Статус:** 🟡 MEDIUM PRIORITY

---

## 🟢 МАЛЕНЬКИЕ ПРОБЛЕМЫ (nice-to-have)

### 8. **Нет валидации на стороне сервера**
Валидация email, password, form fields происходит только на фронте. Нужно убедиться что бэкенд også валидирует.

**Статус:** 🟢 LOW PRIORITY (бэкендер должен это сделать)

---

### 9. **Modal на delete действии не работает**
**Файл:** `/src/components/layout/AppLayout.tsx` (line 68)

```typescript
<button className="btn-danger" onClick={() => {
  setModalDel(false);
  showToast('✓ Акт удалён (требуется backend)')
}}>Удалить</button>
```

Это только показывает toast, не удаляет на самом деле!

**Решение:**
Вызвать правильный delete API через hook.

**Статус:** 🟡 MEDIUM PRIORITY

---

### 10. **Нет обработки edge cases в forms**
- Нет trim() при отправке форм
- Нет проверки на пустые строки
- Нет обработки специальных символов

**Статус:** 🟡 MEDIUM PRIORITY

---

### 11. **localStorage доступ без try-catch**
**Файлы:** `/src/store/index.ts`, `/src/hooks/index.ts`

```typescript
const savedUser = localStorage.getItem('legist_user')  // ← может быть null
```

Если это в приватном браузере или localStorage отключен (кликпреди на сайт), это не будет работать.

**Решение:**
```typescript
const savedUser = (() => {
  try {
    const raw = localStorage.getItem('legist_user')
    if (raw) return JSON.parse(raw)
  } catch {
    // localStorage деактивирован или error
    console.warn('localStorage недоступен')
  }
  return null
})()
```

**Статус:** 🟡 MEDIUM PRIORITY

---

## 📋 ROADMAP - ЧТО НУЖНО СДЕЛАТЬ

### Phase 1: CRITICAL FIXES (1-2 дня) 🔴
- [ ] Исправить SSE авторизацию
- [ ] Исправить parallel upload loading state bug
- [ ] Исправить XSS vulnerability в чате
- [ ] Реализовать refresh token logic

### Phase 2: IMPORTANT FEATURES (3-4 дня) 🟠
- [ ] Добавить network timeout в apiFetch
- [ ] Проверить размер файла перед загрузкой
- [ ] Исправить delete modal (реальное удаление)
- [ ] Улучшить error handling

### Phase 3: POLISH (5-7 дней) 🟡
- [ ] Улучшить форм валидацию
- [ ] Улучшить localStorage handling
- [ ] Добавить больше информативных ошибок
- [ ] Улучшить mobile responsiveness

### Phase 4: FUTURE FEATURES 🟢
- [ ] Implementasi caching для API calls
- [ ] Offline support
- [ ] File drag-and-drop для drop зоны
- [ ] Batch operations на файлах
- [ ] History отката версий
- [ ] Share документов между пользователями

---

## 📌 РЕКОМЕНДАЦИИ НЕМЕДЛЕННЫХ ДЕЙСТВИЙ

### На сегодня:

1. **Спроси у бэкендера:**
   - Как передавать Authorization для SSE?
   - Точно ли endpoint для чата `/api/chat`?
   - Есть ли `POST /api/tokens/refresh` для обновления токенов?

2. **Исправь критические баги (сегодня):**
   - SSE auth
   - Parallel upload loading state
   - Refresh token logic
   - XSS protection в чате

3. **Закоммитьте архитектурный документ:**
   ```bash
   git add ARCHITECTURE.md
   git commit -m "docs: add comprehensive architecture guide"
   git push
   ```

---

## 📊 ТАБЛИЦА ПРИОРИТЕТОВ

| № | Проблема | Приоритет | Сложность | Время | Deadline |
|----|----------|-----------|-----------|-------|----------|
| 1 | SSE Authorization | 🔴 Critical | Medium | 1-2ч | Сегодня |
| 2 | Parallel upload loading | 🔴 Critical | Easy | 30мин | Сегодня |
| 3 | Refresh token | 🔴 Critical | Medium | 2-3ч | Завтра |
| 4 | XSS protection | 🔴 Security | Easy | 1ч | Сегодня |
| 5 | Network timeout | 🟠 Important | Easy | 30мин | Завтра |
| 6 | File size check | 🟠 Important | Easy | 30мин | Завтра |
| 7 | Delete confirmation | 🟠 Important | Easy | 1ч | Завтра |
| 8 | Chat modal | 🟢 Nice-to-have | Medium | 2ч | На неделю |

---

## 🎯 NEXT STEPS

**Немедленно:**
```bash
# 1. Исправить SSE (после ответа бэкендера)
# 2. Исправить parallel upload loading state
# 3. Добавить refresh token logic
# 4. Исправить XSS protection

# После исправлений:
npm run build
# Тестировать с разными сценариями

# Коммитить:
git add .
git commit -m "fix: critical bugs - auth, loading states, refresh tokens, XSS protection"
git push
```

---

**Последнее обновление:** 2026-03-20
**Автор анализа:** Claude Code Assistant
