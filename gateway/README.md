# Gateway

HTTP-шлюз на порту `8080`. Эндпоинты и формат ответов остались прежними, но теперь Ledger хранит данные в PostgreSQL,
поэтому они переживают перезапуск. Отчёт по расходам кешируется в Redis на 30 секунд.
Gateway импортирует только публичный пакет `ledger`: при старте получает `ledger.Service` и `closeFn`
из `ledger.NewService(ctx)`, передаёт сервис в `api.NewRouter(svc, timeout)` и откладывает вызов `closeFn()`.
Подключения к PostgreSQL и Redis собирает фабрика Ledger по переменным окружения из [ledger/README.md](../ledger/README.md).

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
`404`/`405` — неизвестный путь или метод; `500` — ошибка БД; `504` — таймаут или отмена контекста.

## Таймауты запросов

Middleware создаёт `context.WithTimeout` для каждого запроса и передаёт контекст через `r.WithContext(ctx)`.
Таймаут по умолчанию — **2 секунды**, включая все API-ручки и `/ping`.
Переменная `REQUEST_TIMEOUT` задаётся при запуске и принимает положительную длительность Go:

```bash
cd gateway
REQUEST_TIMEOUT=2s go run .
```

Для проверки можно заменить `2s` на `50ms`. Пустая переменная означает значение по умолчанию;
нулевое, отрицательное или некорректное значение завершает запуск с ошибкой.
Ответ буферизуется до окончания обработки. При истечении таймаута или отмене контекста
возвращается `504 Gateway Timeout` с `Content-Type: application/json; charset=utf-8`:

```json
{"error":"request timed out or canceled"}
```

Контекст проходит через HTTP-обработчики, Ledger, PostgreSQL и Redis. Рабочие горутины отчёта
прекращают SQL-операции по отмене, сервис ожидает их завершения и остановки heartbeat.
Поздний результат не изменяет уже отправленный `504`. Если клиент разорвал соединение,
операция отменяется, но клиент уже не сможет получить JSON-ответ.

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

Параметры `from` и `to` обязательны, имеют формат `YYYY-MM-DD`, обе границы включены.
Отсутствующие даты, невозможные даты и `from > to` возвращают `400` с описанием ошибки.
Период без расходов возвращает `{"from":"2026-10-01","to":"2026-10-31","categories":[],"total":0}`.
В отчёт входят категории с расходами за выбранный период.

Эффект кеша виден в логе Gateway:

```
Ledger: отчёта report:summary:2026-10-01:2026-10-31 нет в кеше, посчитан по БД
Ledger: отчёт report:summary:2026-10-01:2026-10-31 взят из кеша
```

Ключ можно посмотреть и в Redis: `redis-cli ttl report:summary:2026-10-01:2026-10-31`.
После перезапуска Gateway запросы `GET /api/transactions` и `GET /api/budgets` возвращают ранее сохранённые данные.

## Проверка отмены расчёта

Используйте отдельную тестовую БД с миграциями и отдельную базу Redis, например `REDIS_DB=15`.
Сначала выполните обычный запрос с `REQUEST_TIMEOUT=2s` и проверьте суммы.
Быстрый запрос может успеть даже за 1–50 мс, особенно при попадании в кеш.
Для воспроизводимой проверки без задержек в коде приложения:

1. Остановите Gateway и запустите его с `REQUEST_TIMEOUT=50ms` и теми же тестовыми подключениями.
2. Удалите только ключ проверяемого отчёта в тестовой базе Redis:

   ```bash
   redis-cli -n 15 DEL report:summary:2026-10-01:2026-10-31
   ```

3. В отдельном терминале подключитесь к **тестовой** БД через `psql "$DATABASE_URL"`
   и оставьте открытой транзакцию:

   ```sql
   BEGIN;
   LOCK TABLE expenses IN ACCESS EXCLUSIVE MODE;
   ```

4. Повторите запрос отчёта. Он вернёт `504` с JSON-ошибкой примерно через 50 мс.
   В логе появятся `расчёт прерван по ctx.Done(): context deadline exceeded` и `heartbeat остановлен`.
   При 50 мс тикер успевает остановиться до первого тика; чтобы увидеть сообщение
   `идёт расчёт отчёта`, повторите блокировку при таймауте больше 400 мс.
5. Проверьте в `pg_stat_activity`, что отменённый `SELECT DISTINCT category ...` больше не активен,
   затем выполните `ROLLBACK;` в терминале с блокировкой.
6. Перезапустите Gateway с `REQUEST_TIMEOUT=2s` или без этой переменной и повторите запрос — ожидается `200`.

Тесты сервиса отдельно проверяют одновременный запуск категорий, завершение всех рабочих горутин
при отмене и ошибке, а также остановку heartbeat до возврата из расчёта.

## Тесты

```bash
export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/cashapp_test?sslmode=disable'
cd gateway
go test -race ./... -count=1
```

Без `TEST_DATABASE_URL` выполняются проверки валидации, конфигурации и middleware (включая `504`
на всех API-ручках, отмену и защиту от позднего ответа); тесты с БД пропускаются.
Тестовая БД готовится так же, как для ledger.
