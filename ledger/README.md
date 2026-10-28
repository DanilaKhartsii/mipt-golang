# Ledger

Пакет с бизнес-логикой учёта расходов: транзакции, бюджеты категорий и отчёт по расходам.
Ledger хранит данные в PostgreSQL, а результаты отчёта и список бюджетов кеширует в Redis на короткий TTL.
Gateway подключает ledger как Go-пакет и вызывает `ledger.Init()` при старте, поэтому отдельный процесс для Ledger не нужен.

## Структура

- `budget.go`, `transaction.go`, `report.go` — бизнес-правила и SQL-запросы;
- `storage.go` — `Init()` и общие подключения к PostgreSQL и Redis;
- `internal/db` — DSN из переменных окружения, `sql.Open("pgx", dsn)`, пул соединений и `Ping()`;
- `internal/cache` — клиент `github.com/redis/go-redis/v9`, значения хранятся в JSON;
- `migrations` — миграции Goose для таблиц `budgets` и `expenses`;
- `cmd/ledger` — консольная демонстрация, работает с той же БД.

## PostgreSQL и Redis

```bash
docker run --name cashcraft-postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=cashapp -p 5432:5432 -d postgres:17-alpine
docker run --name cashcraft-redis -p 6379:6379 -d redis:7-alpine
```

Если PostgreSQL уже установлен локально, достаточно создать базу: `createdb cashapp`.

## Миграции Goose

Установка утилиты:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Если на macOS установка падает с ошибкой линковки (`linker command failed`), повторите её с `CGO_ENABLED=0`.

Миграции применяются перед первым запуском и после каждого изменения схемы. Команды выполняются из корня репозитория:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable'

goose -dir ./ledger/migrations postgres "$DATABASE_URL" up      # применить все миграции
goose -dir ./ledger/migrations postgres "$DATABASE_URL" down    # откатить последнюю миграцию
goose -dir ./ledger/migrations postgres "$DATABASE_URL" status  # посмотреть состояние
```

Порядок запуска: PostgreSQL и Redis → `goose ... up` → Gateway (он инициализирует Ledger).

## Переменные окружения

| Переменная       | По умолчанию     | Описание                                                       |
|------------------|------------------|----------------------------------------------------------------|
| `DATABASE_URL`   | —                | DSN PostgreSQL; если задана, переменные `DB_*` не используются |
| `DB_HOST`        | `localhost`      | Хост PostgreSQL                                                |
| `DB_PORT`        | `5432`           | Порт PostgreSQL                                                |
| `DB_USER`        | `postgres`       | Пользователь                                                   |
| `DB_PASS`        | `postgres`       | Пароль                                                         |
| `DB_NAME`        | `cashapp`        | Имя базы данных                                                |
| `REDIS_ADDR`     | `localhost:6379` | Адрес Redis                                                    |
| `REDIS_DB`       | `0`              | Номер базы Redis                                               |
| `REDIS_PASSWORD` | —                | Пароль Redis                                                   |

При сборке DSN из `DB_*` всегда добавляется `sslmode=disable`.
Пул соединений: `SetMaxOpenConns(10)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(30m)`.

При успешном старте в логе появляются строки:

```
Ledger: подключение к PostgreSQL установлено
Ledger: подключение к Redis localhost:6379 (db 0) установлено
```

Если PostgreSQL или Redis недоступны, приложение выводит причину и завершается с кодом 1.

Консольная демонстрация:

```bash
cd ledger
go run ./cmd/ledger
```

## Бизнес-правила

- Перед записью вызываются `Budget.Validate()` и `Transaction.Validate()`.
- `SetBudget` выполняет UPSERT по `category` и удаляет кеш `budgets:all`.
- `AddTransaction` берёт лимит категории и уже потраченную сумму. Если `spent + amount > limit_amount`,
  возвращается `ErrBudgetExceeded` и запись не вставляется. Категория без бюджета не ограничена.
  ID новой транзакции берётся из `RETURNING id`.
- `ListTransactions` сортирует по `date DESC, id DESC`, `ListBudgets` — по `category`.

## Кеш Redis

| Ключ                         | TTL  | Содержимое                                        |
|------------------------------|------|---------------------------------------------------|
| `report:summary:<from>:<to>` | 30 с | `GetReportSummary`: суммы по категориям за период |
| `budgets:all`                | 30 с | `ListBudgets`; удаляется после `SetBudget`        |

При промахе отчёт считается запросом `SUM(amount) ... GROUP BY category` и сохраняется в Redis.
После добавления транзакции кеш отчёта не сбрасывается, поэтому отчёт может отставать от данных не дольше TTL.
Ошибки Redis во время работы только логируются: данные в этом случае читаются из БД.

## Тесты

Тесты валидации работают без внешних зависимостей. Тестам хранилища нужна отдельная БД с применёнными миграциями.
Без `TEST_DATABASE_URL` они пропускаются. Тесты очищают таблицы через `TRUNCATE`, поэтому рабочую БД указывать нельзя.

```bash
createdb cashapp_test   # или docker exec cashcraft-postgres psql -U postgres -c 'CREATE DATABASE cashapp_test'
export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/cashapp_test?sslmode=disable'
goose -dir ./ledger/migrations postgres "$TEST_DATABASE_URL" up

cd ledger
go test ./... -cover
```
