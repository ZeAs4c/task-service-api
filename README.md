Сервис для управления задачами с поддержкой периодических шаблонов, пагинации и гибкого планирования.  
Разработан в рамках тестового задания на позицию Junior Backend Developer.
---

## Оглавление

- [Возможности](#-возможности)
- [Технологии](#-технологии)
- [Архитектура проекта](#-архитектура-проекта)
- [Быстрый старт](#-быстрый-старт)
  - [Windows (PowerShell)](#windows-powershell)
  - [Linux / macOS (Bash)](#linux--macos-bash)
- [Тестирование API](#-тестирование-api)
  - [Через Swagger UI](#через-swagger-ui)
  - [Через PowerShell (Windows)](#через-powershell-windows)
  - [Через curl (Linux/macOS)](#через-curl-linuxmacos)
- [Описание API](#-описание-api)
- [Примеры запросов](#-примеры-запросов)
- [Особенности реализации](#-особенности-реализации)
- [Что можно улучшить](#-что-можно-улучшить)
- [Тестирование кода](#-тестирование-кода)

---

## Возможности

### Базовый CRUD
- Создание, просмотр, обновление и удаление задач
- Пагинация списка задач (`page`, `page_size`)
- Планирование задач на конкретную дату (`scheduled_at`)

### Периодические задачи (шаблоны)
- **Ежедневные** (`daily`) - каждый N-й день
- **Ежемесячные** (`monthly`) - в определённое число месяца (1–31)
- **По конкретным датам** (`specific_dates`) - список произвольных дат
- **Чётные / нечётные дни** (`parity`) - `even` или `odd`

### Генерация задач
- Создание шаблона (`is_template: true`)
- Ручная генерация задач из шаблона до указанной даты
- Защита от дублирования при повторной генерации
- Обработка граничных случаев

### Документация
- Swagger UI в комплекте
- OpenAPI 3.0 спецификация

---

## Технологии

| Слой | Технологии |
|------|------------|
| Язык | Go 1.23+ |
| Роутер | Gorilla Mux |
| База данных | PostgreSQL 16 |
| Драйвер БД | pgx/v5 |
| Документация | Swagger UI |
| Контейнеризация | Docker + Docker Compose |
| Логирование | slog |
| Архитектура | Clean Architecture |

---

## Архитектура проекта
cmd/
-  api/ # Точка входа, DI

internal/
-  domain/
-  task/ # Бизнес-сущности (Task, RecurrenceRule)
-  usecase/
-  task/ # Сценарии использования (сервисный слой)
-  repository/
-  postgres/ # Реализация репозитория (pgx)
-  infrastructure/
-  postgres/ # Пул соединений с БД
-  transport/
-  http/
-  handlers/ # HTTP хендлеры
-  docs/ # Swagger UI + openapi.json

migrations/ # SQL миграции

**Принципы:**
- Зависимости направлены внутрь (domain - usecase - repository/transport)
- Бизнес-логика изолирована от деталей реализации
- Легко тестировать каждый слой отдельно

---

## Быстрый старт

### Предварительные требования

- **Docker Desktop** (установлен и запущен)
- **Git** (для клонирования)
- **PowerShell** (Windows) или **Terminal** (Linux/macOS)

### Windows (PowerShell)

```powershell
# 1. Клонируем репозиторий
git clone https://github.com/ZeAs4c/task-service-api.git
cd task-service-api

# 2. Запускаем через Docker Compose
docker compose up --build

# 3. Ждём, пока появится:
#    http server started addr=:8080
После запуска открывай браузер:

API: http://localhost:8080

Swagger UI: http://localhost:8080/swagger/

Остановка:

powershell
# Ctrl+C в терминале, затем:
docker compose down

# Если нужно сбросить базу данных полностью:
docker compose down -v
Linux / macOS (Bash)
bash
# 1. Клонируем репозиторий
git clone https://github.com/ZeAs4c/task-service-api.git
cd task-service-api

# 2. Запускаем через Docker Compose
docker compose up --build

# 3. Ждём сообщения:
#    http server started addr=:8080
После запуска:

API: http://localhost:8080

Swagger UI: http://localhost:8080/swagger/

Остановка:

bash
# Ctrl+C, затем:
docker compose down

# Сброс базы:
docker compose down -v
Тестирование API
Через Swagger UI
Самый простой способ - открыть http://localhost:8080/swagger/ и дёргать ручки прямо из браузера.

Через PowerShell (Windows)
Windows не дружит с curl в том виде, как на Linux. Используй Invoke-WebRequest или Invoke-RestMethod.

1. Создать обычную задачу

powershell
$body = @{
    title = "Обход пациентов"
    description = "Утренний обход в терапии"
    scheduled_at = "2026-04-10T09:00:00Z"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tasks" `
    -Method Post `
    -Body $body `
    -ContentType "application/json"
2. Создать шаблон (ежедневная задача)

powershell
$body = @{
    title = "Ежедневный обзвон"
    recurrence_type = "daily"
    recurrence_rule = @{ interval = 1 }
    is_template = $true
} | ConvertTo-Json -Depth 3

Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tasks" `
    -Method Post `
    -Body $body `
    -ContentType "application/json"
3. Получить список задач с пагинацией

powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tasks?page=1&page_size=5" `
    -Method Get
4. Сгенерировать задачи из шаблона

powershell
# Предположим, ID шаблона = 1
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/templates/1/generate?until=2026-04-30" `
    -Method Post
5. Обновить дату задачи

powershell
$body = @{
    title = "Обход пациентов"
    status = "in_progress"
    scheduled_at = "2026-04-15T14:00:00Z"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tasks/1" `
    -Method Put `
    -Body $body `
    -ContentType "application/json"
Через curl (Linux/macOS)
1. Создать обычную задачу

bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Обход пациентов",
    "scheduled_at": "2026-04-10T09:00:00Z"
  }'
2. Создать шаблон (чётные дни)

bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Отчётность",
    "recurrence_type": "parity",
    "recurrence_rule": {"parity": "even"},
    "is_template": true
  }'
3. Сгенерировать задачи

bash
curl -X POST "http://localhost:8080/api/v1/templates/1/generate?until=2026-04-30"
4. Пагинация

bash
curl "http://localhost:8080/api/v1/tasks?page=2&page_size=10"
5. Обновить задачу

bash
curl -X PUT http://localhost:8080/api/v1/tasks/1 \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Обход пациентов",
    "status": "done",
    "scheduled_at": "2026-04-10T12:00:00Z"
  }'
📡 Описание API
Основные эндпоинты
Метод	Путь	Описание
POST	/api/v1/tasks	Создать задачу
GET	/api/v1/tasks	Список задач (с пагинацией)
GET	/api/v1/tasks/{id}	Получить задачу по ID
PUT	/api/v1/tasks/{id}	Обновить задачу
DELETE	/api/v1/tasks/{id}	Удалить задачу
GET	/api/v1/templates	Список всех шаблонов
POST	/api/v1/templates/{id}/generate?until=YYYY-MM-DD	Сгенерировать задачи
Параметры пагинации (GET /api/v1/tasks)
Параметр	Тип	По умолчанию	Максимум
page	int	1	-
page_size	int	20	100
Типы периодичности (recurrence_type)
Тип	Поле в recurrence_rule	Пример
daily	interval (int)	{"interval": 2} — каждый 2-й день
monthly	day_of_month (int, 1–31)	{"day_of_month": 15} — 15-го числа
specific_dates	dates ([]string)	{"dates": ["2026-04-10", "2026-04-20"]}
parity	parity ("even" | "odd")	{"parity": "odd"} — только нечётные дни
💡 Примеры запросов (все варианты)
1. Обычная задача
PowerShell:

powershell
$body = @{ title = "Провести операцию"; scheduled_at = "2026-04-10T08:00:00Z" } | ConvertTo-Json
Invoke-RestMethod -Uri http://localhost:8080/api/v1/tasks -Method Post -Body $body -ContentType "application/json"
Bash:

bash
curl -X POST http://localhost:8080/api/v1/tasks -H "Content-Type: application/json" -d '{"title":"Провести операцию","scheduled_at":"2026-04-10T08:00:00Z"}'
2. Ежедневный шаблон (каждый день)
PowerShell:

powershell
$body = @{
    title = "Утренний обход"
    recurrence_type = "daily"
    recurrence_rule = @{ interval = 1 }
    is_template = $true
} | ConvertTo-Json -Depth 3
Invoke-RestMethod -Uri http://localhost:8080/api/v1/tasks -Method Post -Body $body -ContentType "application/json"
Bash:

bash
curl -X POST http://localhost:8080/api/v1/tasks -H "Content-Type: application/json" -d '{"title":"Утренний обход","recurrence_type":"daily","recurrence_rule":{"interval":1},"is_template":true}'
3. Ежемесячный шаблон (15-го числа)
PowerShell:

powershell
$body = @{
    title = "Инвентаризация"
    recurrence_type = "monthly"
    recurrence_rule = @{ day_of_month = 15 }
    is_template = $true
} | ConvertTo-Json -Depth 3
Invoke-RestMethod -Uri http://localhost:8080/api/v1/tasks -Method Post -Body $body -ContentType "application/json"
Bash:

bash
curl -X POST http://localhost:8080/api/v1/tasks -H "Content-Type: application/json" -d '{"title":"Инвентаризация","recurrence_type":"monthly","recurrence_rule":{"day_of_month":15},"is_template":true}'
4. Шаблон на конкретные даты
PowerShell:

powershell
$body = @{
    title = "Планёрка"
    recurrence_type = "specific_dates"
    recurrence_rule = @{ dates = @("2026-04-10", "2026-04-17", "2026-04-24") }
    is_template = $true
} | ConvertTo-Json -Depth 4
Invoke-RestMethod -Uri http://localhost:8080/api/v1/tasks -Method Post -Body $body -ContentType "application/json"
Bash:

bash
curl -X POST http://localhost:8080/api/v1/tasks -H "Content-Type: application/json" -d '{"title":"Планёрка","recurrence_type":"specific_dates","recurrence_rule":{"dates":["2026-04-10","2026-04-17","2026-04-24"]},"is_template":true}'
5. Генерация задач из шаблона
PowerShell:

powershell
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/templates/1/generate?until=2026-05-01" -Method Post
Bash:

bash
curl -X POST "http://localhost:8080/api/v1/templates/1/generate?until=2026-05-01"

Тестируем в SWAGGER:
Открываем Swagger
В браузере переходим:

http://localhost:8080/swagger/

Создаём шаблон ежедневной задачи
Разверни POST /api/v1/tasks
Нажми Try it out

Вставь это тело запроса:

json
{
  "title": "Ежедневный обход",
  "description": "Утренний обход пациентов",
  "recurrence_type": "daily",
  "recurrence_rule": {
    "interval": 1
  },
  "is_template": true
}
Нажми Execute
Смотри ответ - там будет "id": 1

Создаём ещё шаблон (на чётные дни)
Там же в POST /api/v1/tasks, новое тело:

json
{
  "title": "Отчётность по чётным",
  "description": "Формирование отчёта",
  "recurrence_type": "parity",
  "recurrence_rule": {
    "parity": "even"
  },
  "is_template": true
}
Нажми Execute - получишь "id": 2

Создаём шаблон на конкретные даты
json
{
  "title": "Планёрка",
  "description": "Еженедельное совещание",
  "recurrence_type": "specific_dates",
  "recurrence_rule": {
    "dates": ["2026-04-15", "2026-04-22", "2026-04-29"]
  },
  "is_template": true
}
Нажми Execute - "id": 3

Создаём ежемесячный шаблон (15 число)
json
{
  "title": "Инвентаризация",
  "description": "Ежемесячная проверка",
  "recurrence_type": "monthly",
  "recurrence_rule": {
    "day_of_month": 15
  },
  "is_template": true
}
Execute - "id": 4

Смотрим все шаблоны
Разверни GET /api/v1/templates
Try it out - Execute
Должен вернуться массив из 4 шаблонов

Генерируем задачи из первого шаблона
Разверни POST /api/v1/templates/{id}/generate
Try it out
В поле id введи: 1
В поле until введи: 2026-04-20
Execute
Смотри ответ: должно создаться ~12 задач (с 9 по 20 апреля)

Генерируем задачи из шаблона на чётные дни
Тот же эндпоинт POST /api/v1/templates/{id}/generate
id: 2
until: 2026-04-20
Execute
Должны создаться задачи только на чётные числа: 10, 12, 14, 16, 18, 20 апреля.

Генерируем задачи из шаблона на конкретные даты
id: 3
until: 2026-04-30
Execute
Создадутся задачи на 15, 22, 29 апреля.

Генерируем ежемесячные задачи
id: 4
until: 2026-06-01
Execute
Создадутся задачи на 15 апреля, 15 мая.

Проверяем список всех задач с пагинацией
Разверни GET /api/v1/tasks
Нажми Try it out
Параметры:
page: 1
page_size: 10
Execute

Берём ID любой задачи для обновления
Из списка задач в шаге 12 возьми ID первой задачи (например, 5)

Обновляем задачу (меняем дату и статус)
Разверни PUT /api/v1/tasks/{id}
Нажми Try it out
В поле id введи ID из шага 13 (например, 5)
В тело запроса вставь:

json
{
  "title": "Ежедневный обход (ПЕРЕНОС)",
  "description": "Утренний обход пациентов",
  "status": "in_progress",
  "scheduled_at": "2026-04-25T14:00:00Z"
}
Нажми Execute
Смотри ответ: поле scheduled_at должно измениться на 2026-04-25T14:00:00Z

Проверяем, что задача обновилась
Разверни GET /api/v1/tasks/{id}
Введи тот же ID (5)
Execute

Удаляем задачу
Разверни DELETE /api/v1/tasks/{id}
Введи ID (5)
Нажми Execute
Должен вернуться статус 204 No Content

Проверяем, что задача удалена
Снова GET /api/v1/tasks/{id} с ID 5
Должна вернуться ошибка 404 с сообщением "task not found"

Особенности реализации
1. Хранение правил периодичности
Правила (recurrence_rule) хранятся в PostgreSQL как JSONB, что позволяет:

Гибко расширять типы без изменения схемы

Делать запросы внутрь JSON (если понадобится)

2. Защита от дубликатов
Уникальный индекс uniq_template_date на (parent_template_id, scheduled_at) не даёт создать две одинаковые задачи на одну дату при повторной генерации.

3. Обработка граничных случаев
monthly с числом 31 - для месяцев с 30 днями ставится 30-е, для февраля - последний день

daily с интервалом > 1 - корректно перешагивает через месяцы и годы

specific_dates - игнорирует даты в прошлом и даты позже until

4. Пагинация
Дефолтные значения: page=1, page_size=20

Максимальный размер страницы: 100

Возвращается мета-информация: total, page, page_size

5. Валидация
interval > 0 для daily

day_of_month от 1 до 31 для monthly

Даты в dates строго в формате YYYY-MM-DD

parity только "even" или "odd"

Что можно улучшить (production-ready)
Ближайшие улучшения
Автоматическая генерация по крону - сейчас генерация только по запросу. Можно добавить фоновый процесс, который раз в сутки создаёт задачи на завтра.

Часовые пояса - добавить поле timezone в шаблон и учитывать локальное время врача.

Фильтрация и сортировка - в GET /tasks добавить status, date_from, date_to, sort_by, sort_order.

Soft delete - вместо физического удаления ставить флаг deleted_at.

Версионирование API - добавить /api/v2 для обратной совместимости при крупных изменениях.

Архитектурные улучшения
Graceful shutdown для БД - добавить ожидание завершения транзакций перед закрытием пула.

Миграции через goose/golang-migrate - сейчас SQL-файлы монтируются в Docker, лучше использовать инструмент миграций.

Кеширование - Redis для хранения сгенерированных задач на день вперёд.

Метрики и трейсинг - Prometheus + Jaeger для мониторинга.

Безопасность
Аутентификация - JWT токены, роли (врач/админ).

Rate limiting - защита от DDoS.

Валидация входных данных - более строгая, с кастомными сообщениями об ошибках.

Тестирование кода
Запуск всех тестов
Windows (PowerShell):

powershell
go test ./...
Linux/macOS:

bash
go test ./...
Запуск тестов с подробным выводом
bash
go test ./... -v
Запуск тестов конкретного пакета
bash
go test ./internal/usecase/task -v
Покрытие тестами
bash
go test ./... -cover
Текущие тесты покрывают:

- Создание задачи с валидацией

- Создание шаблона с периодичностью

- Генерацию задач из шаблона

- Пагинацию

- Обновление scheduled_at

Примечания
О часовых поясах
Все даты хранятся и обрабатываются в UTC. При отправке запросов указывайте время с суффиксом Z или смещением, например:

2026-04-10T09:00:00Z (UTC)

2026-04-10T12:00:00+03:00 (Москва)

О инициализации БД
Миграции применяются автоматически при первом запуске Docker Compose. Если вы меняли схему, пересоздайте volume:

bash
docker compose down -v
docker compose up --build

