# Gateway

HTTP-шлюз на порту `8080`. Эндпоинты и формат ответов остались прежними, но теперь Ledger хранит данные в PostgreSQL,
поэтому они переживают перезапуск. Отчёт по расходам кешируется в Redis на 30 секунд.
Gateway импортирует пакет `ledger` и при старте вызывает `ledger.Init()`: подключается к PostgreSQL и Redis
по переменным окружения из [ledger/README.md](../ledger/README.md).

## Запуск

1. Поднимите PostgreSQL и Redis, создайте БД `cashapp` (см. [ledger/README.md](../ledger/README.md)).
2. Примените миграции из корня репозитория:

   ```bash
   export DATABASE_URL='postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable'
   goose -dir ./ledger/migrations postgres "$DATABASE_URL" up
   ```

3. Запустите Gateway:

   ```bash
   cd gateway
   DATABASE_URL="$DATABASE_URL" REDIS_ADDR=localhost:6379 REDIS_DB=0 REDIS_PASSWORD= go run .
   ```

В логе должны появиться строки о подключении к PostgreSQL и Redis, а затем `Gateway service started on :8080`.

## Эндпоинты

| Метод  | Путь                                                 | Описание                             | Успех         |
|--------|------------------------------------------------------|--------------------------------------|---------------|
| `POST` | `/api/transactions`                                  | Создать транзакцию                   | `201 Created` |
| `GET`  | `/api/transactions`                                  | Список транзакций (`[]`, если пусто) | `200 OK`      |
| `POST` | `/api/budgets`                                       | Создать или обновить бюджет          | `201 Created` |
| `GET`  | `/api/budgets`                                       | Список бюджетов                      | `200 OK`      |
| `GET`  | `/api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD` | Расходы по категориям за период      | `200 OK`      |
| `GET`  | `/ping`                                              | Проверка доступности                 | `200 OK`      |

Ошибки: `400` — некорректный JSON, параметры или ошибка валидации; `409` — `{"error":"budget exceeded"}`;
`404`/`405` — неизвестный путь или метод; `500` — ошибка БД.

## Проверка через cURL

```bash
# 201, запись появляется в таблице budgets
curl -i -X POST http://localhost:8080/api/budgets \
  -d '{"category":"еда","limit":5000}'

# 201 — в пределах лимита, запись появляется в expenses
curl -i -X POST http://localhost:8080/api/transactions \
  -d '{"amount":1500,"category":"еда","description":"продукты","date":"2026-10-26"}'

# 409 — превышение лимита, запись не вставляется
curl -i -X POST http://localhost:8080/api/transactions \
  -d '{"amount":4000,"category":"еда","description":"ресторан","date":"2026-10-27"}'

# Списки из БД
curl -i http://localhost:8080/api/transactions
curl -i http://localhost:8080/api/budgets

# Отчёт дважды подряд: первый запрос считается по БД, второй берётся из Redis
curl -i 'http://localhost:8080/api/reports/summary?from=2026-10-01&to=2026-10-31'
curl -i 'http://localhost:8080/api/reports/summary?from=2026-10-01&to=2026-10-31'
```

Ответ отчёта:

```json
{"from":"2026-10-01","to":"2026-10-31","categories":[{"category":"еда","total":1500}],"total":1500}
```

Эффект кеша виден в логе Gateway:

```
Ledger: отчёта report:summary:2026-10-01:2026-10-31 нет в кеше, посчитан по БД
Ledger: отчёт report:summary:2026-10-01:2026-10-31 взят из кеша
```

Ключ можно посмотреть и в Redis: `redis-cli ttl report:summary:2026-10-01:2026-10-31`.
После перезапуска Gateway запросы `GET /api/transactions` и `GET /api/budgets` возвращают ранее сохранённые данные.

## Тесты

```bash
export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/cashapp_test?sslmode=disable'
cd gateway
go test ./... -cover
```

Без `TEST_DATABASE_URL` выполняются только проверки ответов `400`, тесты с БД пропускаются.
Тестовая БД готовится так же, как для ledger.
