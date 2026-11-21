# Go — домашние задания

Требуется Go 1.26+.

## hw-1

```bash
go run ./hw-1 arg1 arg2
```

## hw-2

Два независимых сервиса, у каждого свой `go.mod`:

- `hw-2/gateway` — HTTP-шлюз, порт `8080`, обработчик `GET /ping`.
- `hw-2/ledger` — бизнес-логика: хранилище транзакций в памяти, `AddTransaction` и `ListTransactions`.

### Gateway

```bash
cd hw-2/gateway
go run .
```

Проверка (в другом терминале или в браузере по адресу http://localhost:8080/ping):

```bash
curl -i http://localhost:8080/ping
```

Ожидаемый ответ — `200 OK` с телом `pong`.

### Ledger

```bash
cd hw-2/ledger
go run .
```

Сервис выводит `Ledger service started`, устанавливает бюджет «Еда» через `SetBudget`,
загружает остальные бюджеты из `budgets.json` через `LoadBudgets`, добавляет тестовые транзакции
и печатает бюджеты и список транзакций.
Транзакция с нулевой суммой и транзакции, превышающие бюджет категории, отклоняются с ошибкой
и в список не попадают.

## hw-5

REST API в сервисе `gateway`. Gateway импортирует пакет `ledger` из соседнего модуля
(`require ledger` + `replace ledger => ../ledger` в `gateway/go.mod`), хранилище — в памяти.

- `ledger` — пакет с бизнес-логикой (`Transaction`, `Budget`, `AddTransaction`, `SetBudget` и т.д.);
  консольная демонстрация из прошлых ДЗ перенесена в `ledger/cmd/ledger`.
- `gateway/internal/api` — HTTP-слой: DTO и преобразователи, обработчики, middleware, маршруты.

### Запуск

```bash
cd gateway
go run .
```

Сервер слушает порт `8080`. Консольная демонстрация ledger запускается так:

```bash
cd ledger
go run ./cmd/ledger
```

### Эндпоинты

| Метод  | Путь                | Описание                             | Успех         |
|--------|---------------------|--------------------------------------|---------------|
| `POST` | `/api/transactions` | Создать транзакцию                   | `201 Created` |
| `GET`  | `/api/transactions` | Список транзакций (`[]`, если пусто) | `200 OK`      |
| `POST` | `/api/budgets`      | Создать или обновить бюджет          | `201 Created` |
| `GET`  | `/api/budgets`      | Список бюджетов                      | `200 OK`      |
| `GET`  | `/ping`             | Проверка доступности                 | `200 OK`      |

Все ответы имеют заголовок `Content-Type: application/json; charset=utf-8`.
Ошибки возвращаются в формате `{"error":"..."}`:

- `400 Bad Request` — некорректный JSON или ошибка валидации;
- `409 Conflict` — `{"error":"budget exceeded"}`, транзакция превышает бюджет категории;
- `404 Not Found` / `405 Method Not Allowed` — неизвестный путь или метод;
- `500 Internal Server Error` — прочие внутренние ошибки.

## hw-6

Тесты на стандартной библиотеке (`testing`, `net/http/httptest`), без внешних зависимостей.

- `ledger/ledger_test.go` — unit-тесты домена: табличные проверки `Transaction.Validate()` и `Budget.Validate()`,
  сценарии добавления транзакций с бюджетом категории (в пределах лимита, ровно лимит, превышение, другая категория).
- `gateway/internal/api/handlers_test.go` — интеграционные тесты HTTP-слоя через `NewRouter(svc)`:
  `POST/GET /api/budgets`, `POST/GET /api/transactions`, маппинг ошибок `400` и `409`, заголовок `Content-Type`.

Тесты с БД используют `ledger/testkit`: каждый сценарий очищает отдельную тестовую БД
в начале и в `t.Cleanup()`.

### Запуск тестов

`ledger` и `gateway` — отдельные модули, тесты запускаются в каждом из них:

```bash
cd ledger
go test ./... -cover -coverprofile=cover.out
go tool cover -html=cover.out -o cover.html

cd ../gateway
go test ./... -cover -coverprofile=cover.out
go tool cover -html=cover.out -o cover.html
```

Подробный отчёт по подтестам — с флагом `-v`.

Покрытие зависит от доступности тестовой БД: без `TEST_DATABASE_URL` интеграционные сценарии пропускаются.

## hw-7

Ledger хранит данные в PostgreSQL вместо памяти, а отчёты кеширует в Redis на короткий TTL.
Gateway обращается к тем же HTTP-эндпоинтам с прежними контрактами. Добавлен только `GET /api/reports/summary`
для проверки кеша, а при старте фабрика Ledger подключает PostgreSQL и Redis.

- `ledger/migrations` — миграции Goose для таблиц `budgets` и `expenses`;
- `ledger/internal/db`, `ledger/internal/cache` — подключение к PostgreSQL (`pgx`) и Redis (`go-redis/v9`);
- операции сервиса работают через доменные интерфейсы и SQL-репозитории,
  `GetReportSummary` кеширует результат в `report:summary:<from>:<to>` на 30 секунд.

Порядок запуска: PostgreSQL и Redis → `goose -dir ./ledger/migrations postgres "$DATABASE_URL" up` → Gateway.
Подробности, переменные окружения и примеры cURL — в [ledger/README.md](ledger/README.md) и [gateway/README.md](gateway/README.md).

Тестам хранилища нужна отдельная БД в `TEST_DATABASE_URL`; без неё они пропускаются.

## hw-8

Ledger разделён на `internal/domain` (сущности и контракты), `internal/repository/pg` (SQL),
`internal/service` (правила и интерфейс приложения) и `internal/app` (сборка зависимостей).
Gateway получает готовый `ledger.Service` через `ledger.NewService(ctx)`, передаёт его HTTP-обработчикам
и закрывает подключения через возвращённую фабрикой функцию. Миграции выполняются отдельно.
Проверка лимита и вставка расхода выполняются атомарно с блокировкой категории в PostgreSQL.

## hw-9

`GET /api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD` возвращает расходы по категориям
за период, включая обе даты. При промахе кеша Ledger получает категории из PostgreSQL,
считает каждую в отдельной горутине и собирает результат после `sync.WaitGroup.Wait()`.
Во время расчёта heartbeat пишет в лог раз в 400 мс. Отмена контекста прерывает SQL-запросы;
сервис ожидает завершения всех рабочих горутин и heartbeat.

```bash
curl -i 'http://localhost:8080/api/reports/summary?from=2026-10-01&to=2026-10-31'
```

Пример ответа `200 OK`:

```json
{"from":"2026-10-01","to":"2026-10-31","categories":[{"category":"еда","total":1500},{"category":"транспорт","total":300}],"total":1800}
```

Отсутствующие или некорректные даты, а также `from > to`, возвращают `400` с описанием ошибки.
Пустой период возвращает `categories: []` и `total: 0`. Существующий формат отчёта сохранён.

### Таймауты запросов

Gateway ограничивает каждый запрос, включая все `/api/*`, контекстом с таймаутом **2 секунды**.
Значение меняется переменной окружения `REQUEST_TIMEOUT`, например `REQUEST_TIMEOUT=50ms`
или `REQUEST_TIMEOUT=3s`, при запуске Gateway. Нулевое, отрицательное или некорректное значение
не позволяет серверу запуститься.

При истечении времени или отмене контекста API возвращает **504 Gateway Timeout**:

```json
{"error":"request timed out or canceled"}
```

Если клиент уже разорвал соединение, доставить ему ответ невозможно, но отмена передаётся в Ledger.
Настройки, запуск и воспроизводимая проверка таймаута описаны в [gateway/README.md](gateway/README.md).

## hw-10

`POST /api/transactions/bulk?workers=4` импортирует JSON-массив транзакций конкурентно.
Формат элемента совпадает с одиночным `POST /api/transactions`: `amount`, `category`,
необязательное `description` и `date` в формате `YYYY-MM-DD`.
Параметр `workers` необязателен: по умолчанию **4**, допустимы целые числа от **1 до 64**.
Фактически запускается не больше воркеров, чем транзакций в массиве.

```bash
curl -i -X POST 'http://localhost:8080/api/transactions/bulk?workers=4' \
  -H 'Content-Type: application/json' \
  -d '[
    {"amount":1500,"category":"импорт","description":"продукты","date":"2026-11-01"},
    {"amount":0,"category":"импорт","date":"2026-11-01"}
  ]'

# Тот же эндпоинт с числом воркеров по умолчанию и данными из файла
curl -i -X POST 'http://localhost:8080/api/transactions/bulk' \
  -H 'Content-Type: application/json' --data-binary @transactions.json
```

Если для категории «импорт» нет ограничивающего бюджета, первый запрос возвращает `200 OK`:

```json
{"accepted":1,"rejected":1,"errors":[{"index":1,"error":"сумма транзакции должна быть больше 0"}]}
```

Индексы отсчитываются с **0** в исходном массиве, ошибки отсортированы по индексу.
Ошибки даты, валидации, превышения бюджета и сохранения отдельных транзакций входят в сводку;
при частичном или полном отклонении элементов код остаётся `200`.
Пустой массив возвращает `{"accepted":0,"rejected":0,"errors":[]}`.
Некорректный JSON, неверные типы полей, неизвестные поля, тело не в виде массива
или недопустимый `workers` возвращают `400` до начала импорта.

Ledger передаёт задания через `jobs`, запускает пул горутин и получает результаты через `results`.
Отдельная горутина-коллектор накапливает ошибки и увеличивает счётчики через `atomic.AddInt64`.
После отправки заданий закрывается `jobs`; после `sync.WaitGroup.Wait()` закрывается `results`.
Функция ожидает завершения коллектора перед возвратом. Воркеры переиспользуют `AddTransaction`:
проверка бюджета и запись выполняются в одной SQL-транзакции под блокировкой категории.

При отмене контекста или таймауте Ledger прекращает выдавать задания и возвращает частичную
сводку вместе с `context.Canceled` или `context.DeadlineExceeded`. Завершённые попытки учтены,
не начатые элементы в сводку не входят. Gateway сохраняет контракт ДЗ 9:
`504 Gateway Timeout` и `{"error":"request timed out or canceled"}`.
Успешно сохранённые транзакции остаются в БД; весь пакет не является одной SQL-транзакцией.
Порядок обработки конкурентный: при недостаточном бюджете заранее не определено, какие элементы будут приняты.

### Проверка гонок

В репозитории отдельные Go-модули, поэтому тесты нужно запускать в каждом из них.
С общей тестовой БД запускайте команды последовательно:

```bash
export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/cashapp_test?sslmode=disable'
(cd ledger && go test -race ./... -count=1)
(cd gateway && go test -race ./... -count=1)
```

Подготовка отдельной тестовой БД описана в [ledger/README.md](ledger/README.md).
Без `TEST_DATABASE_URL` проверки PostgreSQL пропускаются; проверки пула, коллектора,
отмены, частичной сводки и HTTP-контракта выполняются без БД.
