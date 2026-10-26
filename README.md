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
- `gateway/internal/api/handlers_test.go` — интеграционные тесты HTTP-слоя через `NewRouter()`:
  `POST/GET /api/budgets`, `POST/GET /api/transactions`, маппинг ошибок `400` и `409`, заголовок `Content-Type`.

Хранилище ledger общее для всех тестов, поэтому тесты, которые его меняют, вызывают `ledger.Reset()`
в начале каждого подтеста и в `t.Cleanup()`.

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

### Покрытие

| Пакет                  | Покрытие |
|------------------------|----------|
| `ledger`               | 71.2%    |
| `gateway/internal/api` | 88.4%    |

Пакеты `main` (`ledger/cmd/ledger`, `gateway`) тестами не покрыты.
