# Go — домашние задания

Требуется Go 1.26+.

## Интерфейс приложения

После запуска Gateway откройте **http://localhost:8080/**: здесь можно задать бюджет,
добавить расход, отправить пакет транзакций и увидеть, какие записи сохранились, а какие были отклонены.
В редакторе уже есть пример; пояснение под ним помогает проверить ограничение бюджета.
Страница работает с настоящим API и PostgreSQL, без отдельного сервера фронтенда и npm-сборки.
Порядок запуска PostgreSQL, Redis и Gateway описан в [gateway/README.md](gateway/README.md).

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

## hw-11 — Docker и Compose

`gateway/Dockerfile` и `ledger/Dockerfile` используют два этапа: Go 1.26 для сборки
(`CGO_ENABLED=0`, `-ldflags="-s -w"`) и Alpine 3.23 для запуска от непривилегированного пользователя.
Версия Go соответствует `go 1.26.3` в модулях. В образе Ledger запускается существующая
консольная демонстрация `cmd/ledger`, рядом с бинарником находится её `budgets.json`.

В Compose работают `db` (PostgreSQL 18), `redis` и `gateway`. Redis нужен существующей
фабрике Ledger и кешу из предыдущих заданий. Отдельного сервиса `ledger` пока нет:
Gateway по-прежнему импортирует его как библиотеку; отдельный контейнер появится в ДЗ 12.
Все три сервиса имеют healthcheck, Gateway ждёт готовности PostgreSQL и Redis.

### Подготовка и сборка образов

Нужны запущенный Docker, BuildKit/Buildx, Docker Compose 2.17+ и Goose
(установка описана в [ledger/README.md](ledger/README.md#миграции-goose)).
Все команды ниже выполняются **из корня репозитория**.

Из-за `replace ledger => ../ledger` сборке Gateway нужны исходники соседнего модуля.
Основной контекст остаётся `./gateway`, а Ledger передаётся через дополнительный контекст
`ledger`: в Compose он задан через `build.additional_contexts`, в CLI — через `--build-context`.
Это передача исходников библиотеки, а не запуск или сборка отдельного сервиса Ledger.

```bash
docker build --build-context ledger=./ledger -t cashcraft-gateway:hw11 ./gateway
docker build -t cashcraft-ledger:hw11 ./ledger
```

Для пересборки Gateway средствами Compose можно использовать `docker compose build gateway`.

### Запуск PostgreSQL и применение миграций

1. Запустите PostgreSQL и дождитесь `healthy` в столбце `STATUS`:

   ```bash
   docker compose up -d db
   docker compose ps db
   ```

   Порт по умолчанию — `127.0.0.1:5432`. Если он занят локальной БД, перед запуском
   выполните `export POSTGRES_PORT=55432` и используйте ту же переменную во всех шагах.
   Альтернатива ручному ожиданию: `docker compose up -d --wait db`.

2. Примените существующие миграции Goose через опубликованный порт:

   ```bash
   export DATABASE_URL="postgres://postgres:postgres@localhost:${POSTGRES_PORT:-5432}/cashapp?sslmode=disable"
   goose -dir ./ledger/migrations postgres "$DATABASE_URL" up
   goose -dir ./ledger/migrations postgres "$DATABASE_URL" status
   ```

   С хоста БД доступна через `localhost`, внутри Compose — через `db:5432`.
   В `docker-compose.yml` Gateway получает именно адрес `db`, а Redis — `redis:6379`.
   Имя базы во всех шагах — `cashapp`. Миграции запускаются явно, отдельного сервиса для них нет.

3. Запустите Gateway; Compose также запустит Redis и дождётся его готовности:

   ```bash
   docker compose up -d gateway
   docker compose ps
   docker compose logs gateway
   ```

   После сборки образа на предыдущем шаге этот запуск использует `cashcraft-gateway:hw11`.
   Дождитесь `healthy` у Gateway. В логах должны быть подключения к PostgreSQL и Redis
   и сообщение `Gateway service started on :8080`.

### Проверка API и записи в БД

```bash
# С хоста: 200 OK и {"message":"pong"} — существующий формат API
curl -i http://localhost:8080/ping

# Из контейнера: тот же запрос используется healthcheck
docker compose exec gateway wget -qO- http://localhost:8080/ping
docker compose exec gateway wget -qO- http://localhost:8080/api/budgets

# Бюджет: 201 Created, затем список с созданной категорией
curl -i -X POST http://localhost:8080/api/budgets \
  -H 'Content-Type: application/json' \
  -d '{"category":"docker-demo","limit":5000}'
curl -i http://localhost:8080/api/budgets

# Транзакция: 201 Created
curl -i -X POST http://localhost:8080/api/transactions \
  -H 'Content-Type: application/json' \
  -d '{"amount":350,"category":"docker-demo","description":"Обед","date":"2026-11-01"}'

# Пакет: 200 OK, accepted=1, rejected=1 (вторая сумма невалидна)
curl -i -X POST 'http://localhost:8080/api/transactions/bulk?workers=4' \
  -H 'Content-Type: application/json' \
  -d '[{"amount":150,"category":"docker-demo","date":"2026-11-02"},{"amount":0,"category":"docker-demo","date":"2026-11-02"}]'

curl -i http://localhost:8080/api/transactions
curl -i 'http://localhost:8080/api/reports/summary?from=2026-11-01&to=2026-11-30'

# Проверка фактически сохранённых строк в PostgreSQL
docker compose exec db psql -U postgres -d cashapp \
  -c "SELECT category, limit_amount FROM budgets WHERE category = 'docker-demo';" \
  -c "SELECT amount, category, date FROM expenses WHERE category = 'docker-demo' ORDER BY id;"
```

При первом выполнении на пустой БД сохраняются две транзакции на общую сумму 500,
отчёт возвращает `total: 500`. Повторный запуск POST создаёт новые расходы и учитывает
уже потраченную сумму. Отчёт кешируется на 30 секунд, как в предыдущих заданиях.

### Отдельный запуск Gateway через docker run

После применения миграций запустите Redis и освободите порт 8080, остановив Gateway в Compose:

```bash
docker compose up -d --wait db redis
docker compose stop gateway
docker run --rm --name cashcraft-gateway-manual --network cashcraft_default \
  -p 8080:8080 \
  -e DATABASE_URL='postgres://postgres:postgres@db:5432/cashapp?sslmode=disable' \
  -e REDIS_ADDR=redis:6379 \
  cashcraft-gateway:hw11
```

В другом терминале выполните `curl -i http://localhost:8080/ping` — ожидается `200 OK`
с JSON-ответом `{"message":"pong"}`. Имя сети `cashcraft_default` задаётся именем проекта `cashcraft`
в Compose; если вы переопределили имя проекта через `-p`, замените и имя сети.
Завершите ручной запуск через Ctrl+C, затем верните сервис командой `docker compose up -d gateway`.
Ledger в этом задании достаточно собрать; его консольная демонстрация не является HTTP-сервером.

### Остановка и сохранность данных

```bash
docker compose down
```

Данные PostgreSQL остаются в именованном томе `postgres_data`, подключённом к
`/var/lib/postgresql` (путь официального образа PostgreSQL 18). При следующем запуске
бюджеты и транзакции сохранятся. Не добавляйте `-v`, если данные нужно сохранить.
