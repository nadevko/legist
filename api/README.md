Мы разрабатываем бэкенд AI-ассистента для сравнения редакций нормативных правовых актов (НПА) Республики Беларусь. Хакатон БГУИР, дедлайн 20.03.2026.

## Структура чанка в Qdrant
payload {
  text    string  — чистый текст чанка
  source  string  — название документа
  article string  — статья/пункт/часть
  level   int     — приоритет в иерархии (0=Конституция ... 9=Технические НПА)
}

## Иерархия НПА (level)
0 — Конституция РБ
1 — Решения республиканских референдумов
2 — Законы РБ
3 — Декреты и указы Президента
4 — Постановления Совета Министров
5 — Постановления Парламента, НПА Верховного суда, Генпрокуратуры
6 — НПА министерств
7 — Местные НПА
8 — НПА иных нормотворческих органов
9 — Технические НПА

## Формат отчёта (JSON)
{
  "id": "uuid",
  "status": "done",
  "summary": "...",
  "diff": [
    {
      "section": "п. 3.1",
      "old_text": "...",
      "new_text": "...",
      "change_type": "semantic|wording|structural",
      "legal_impact": "...",
      "severity": "high|medium|low"
    }
  ],
  "red_zones": [
    {
      "section": "п. 3.1",
      "reference": "Закон №200-З, ст. 45, ч. 2",
      "explanation": "...",
      "severity": "high|medium|low",
      "level": 2
    }
  ]
}

Язык и именование

    Код и комментарии — английский
    Общение в команде — русский
    ID с префиксом типа: file_a1b2c3d4e5f6, user_, sess_, whep_, pwdr_
    12 символов после префикса (срез UUID без дефисов)

API стиль — Stripe

    Ресурсы во множественном числе, ownership через JWT токен
    Каждый ответ содержит id, object, created (unix timestamp)
    Список: {object: "list", data: [...], has_more: bool}
    Удаление: {id, object, deleted: true}
    Ошибки: {object: "error", error: {type, code, message, param?}}
    DELETE возвращает 200 с объектом, не 204
    Cursor-based пагинация через starting_after/ending_before
    expand[]=resource для подгрузки связанных объектов
    Idempotency-Key обязателен для POST

Заголовки

    Request-Id — в каждом ответе
    Legist-Version: v1-alpha — в каждом ответе
    Legist-Signature: sha256=... — в webhook доставках
    Accept — управляет форматом и режимом: application/json / text/event-stream / application/pdf|docx

Go код

    Ошибки через fmt.Errorf("context: %w", err)
    HTTP ошибки через errorf(status, code, message, param?) — никогда echo.NewHTTPError со строкой напрямую
    Конфиг через env + godotenv / direnv
    SQLite через sqlx + modernc.org/sqlite (pure Go, без CGO)
    ID генерация через newID("prefix") в пакете api

База данных — SQLite

    3НФ
    WAL режим, foreign_keys=ON, busy_timeout=5000
    Cursor пагинация через (created_at, id) составной курсор
    Публичные файлы (законы) — user_id IS NULL
    Транзакции для операций затрагивающих несколько таблиц

Асинхронность и стриминг

    Парсинг файлов — горутина, прогресс через SSE broker
    POST /files + Accept: text/event-stream — синхронный режим (стримим прогресс)
    POST /files + Accept: application/json — асинхронный (возвращаем pending)
    GET /files/:id + Accept: text/event-stream — подписаться на статус в любой момент

Webhooks

    HMAC-SHA256 подпись тела запроса
    Exponential backoff: 3 попытки (1s, 4s)
    События хранятся в отдельной таблице webhook_endpoint_events
    Секрет формата whsec_...

Парсер документов

    DOCX: fumiama/go-docx — стили заголовков дают иерархию
    PDF: pdftotext (poppler) через exec — плоский текст + регулярки
    Валидация magic bytes до сохранения на диск
    Лимит 50MB
    Результат: Document{Title, Sections[]{ID, Label, Text, Level, Children}}
    ID секций иерархические: s1, s1.2, s1.2.3

Структура проекта

api/
  cmd/server/main.go
  internal/
    api/        — Echo хендлеры, middleware, типы ответов
    auth/       — JWT, bcrypt, middleware
    config/     — конфиг из env
    pagination/ — cursor-based пагинация
    parser/     — парсинг docx/pdf → Document
    sse/        — SSE broker и стриминг
    store/      — sqlx store по типу ресурса
    webhook/    — dispatcher, подпись, события

NixOS

    Ollama с package = pkgs.ollama-rocm, rocmOverrideGfx = "10.3.5"
    Модели: nomic-embed-text, qwen2.5:3b, qwen2.5:7b
    Qdrant на 127.0.0.1:6333/6334, WAL на диске
    poppler_utils в shell.nix для pdftotext
    gomod2nix для Nix сборки
    Swagger генерация через nix run ..#swagen
