# legist — React App

## Быстрый старт на Mac в PyCharm

### 1. Установить Node.js (если нет)
```bash
# Через Homebrew (рекомендуется)
brew install node

# Проверить
node --version   # должно быть 18+
npm --version
```

### 2. Открыть проект в PyCharm
1. Открой PyCharm
2. File → Open → выбери папку `lexdiff-react`
3. PyCharm увидит `package.json` и предложит установить зависимости — **нажми Install**

   Или вручную в терминале (внизу PyCharm, вкладка Terminal):
   ```bash
   npm install
   ```

### 3. Запустить dev-сервер
В терминале PyCharm:
```bash
npm run dev
```
Открой браузер: **http://localhost:5173**

### 4. Настроить Run Configuration в PyCharm (необязательно, но удобно)
1. Вверху справа → Add Configuration
2. Нажми `+` → npm
3. **package.json**: выбери `package.json` проекта
4. **Command**: `run`
5. **Scripts**: `dev`
6. OK → нажми зелёный треугольник ▶

Теперь можно запускать одной кнопкой.

---

## Структура проекта

```
src/
├── main.tsx              # Точка входа
├── App.tsx               # Роутер
├── styles/
│   └── globals.css       # Все CSS переменные и стили
├── types/
│   └── index.ts          # TypeScript типы
├── data/
│   └── index.ts          # Мок-данные (ACTS_DATA, CHANGES_DATA и т.д.)
├── utils/
│   └── helpers.ts        # rBdg, rFull, pl и другие утилиты
├── store/
│   └── index.ts          # Zustand: authStore, uiStore, compareStore
├── hooks/
│   └── index.ts          # useFileUpload, useCompareProgress, useAssistant
├── components/
│   └── layout/
│       ├── Sidebar.tsx
│       └── AppLayout.tsx  # AuthGuard + Layout + Toast + Modals
└── pages/
    ├── AuthPage.tsx       # Авторизация / Регистрация
    ├── ActsPage.tsx       # Список актов (грид/список)
    └── index.tsx          # ActDetail, Home, Compare, Chain, Assistant
```

## Маршруты

| URL | Страница |
|-----|----------|
| `/auth` | Вход / Регистрация |
| `/` | Главная (drag&drop + последние сравнения) |
| `/acts` | Список актов |
| `/acts/:id` | Детали акта + таблица версий |
| `/compare/:jobId` | Анализ изменений |
| `/chain/:actId` | Цепочка редакций |
| `/assistant` | Чат-ассистент |

## Подключение бэкенда

Все точки интеграции помечены комментарием `// BACKEND:` в коде.

### Авторизация (`src/store/index.ts`)
```ts
// Замени симуляцию на реальный запрос:
const res = await fetch('/api/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password, name })
})
const data = await res.json()
if (!res.ok) throw new Error(data.message)
localStorage.setItem('legist_token', data.token)
```

### Загрузка файлов (`src/hooks/index.ts`)
```ts
const formData = new FormData()
formData.append('file', file)
formData.append('side', side)
const res = await fetch('/api/upload', { method: 'POST', body: formData })
```

### Запуск сравнения (`src/pages/index.tsx` — HomePage)
```ts
// POST /api/compare { oldFileId, newFileId }
// Получаем jobId → polling GET /api/compare/:jobId/status
// При status === 'done' → navigate('/compare/' + jobId)
```

## Билд для продакшн

```bash
npm run build
# Файлы появятся в папке dist/
```
