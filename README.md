# Withdrawals Service

Сервис реализует безопасное создание заявок на вывод средств с идемпотентностью, защитой от двойного списания и подтверждением вывода.

## Локальный запуск

```bash
docker compose up --build
```

Сервис будет доступен на `http://localhost:18080`.

Переменные окружения API:

- `DATABASE_URL`
- `API_TOKEN`
- `HTTP_ADDRESS` (по умолчанию `:18080`)

В `docker-compose.yml` уже выставлены локальные значения:

- токен: `local-dev-token`
- база: `postgres://postgres:postgres@localhost:15432/withdrawals?sslmode=disable`

## Структура проекта

- `cmd/api` содержит entrypoint приложения
- `internal/app` содержит bootstrap и wiring зависимостей
- `internal/domain/withdrawal` содержит доменные типы и правила валидации
- `internal/service/withdrawals` содержит application service
- `internal/repository/postgres` содержит PostgreSQL-реализацию репозитория и транзакционную логику
- `internal/transport/httpapi` содержит HTTP transport, middleware, `healthz/readyz` и error mapping
- `internal/platform/database` содержит встроенные SQL-миграции
- `api/openapi.yaml` содержит OpenAPI 3.0 спецификацию
- `tests/integration` содержит black-box integration tests
- `tests/unit` содержит unit tests по слоям

## Что реализовано

- `POST /v1/withdrawals`
- `GET /v1/withdrawals/{id}`
- `POST /v1/withdrawals/{id}/confirm`
- Bearer auth через `API_TOKEN`
- `GET /healthz`
- `GET /readyz`
- PostgreSQL-транзакции с блокировкой баланса
- idempotency replay для успехов и бизнес-ошибок
- простой ledger в `ledger_entries`
- структурные JSON-логи create/fail/confirm
- встроенные миграции на старте приложения

## Ключевые решения

- Консистентность обеспечивается транзакцией PostgreSQL и `SELECT ... FOR UPDATE` по строке `accounts`.
- Это сериализует конкурентные списания с одного баланса и не позволяет двум запросам одновременно потратить один и тот же остаток.
- Идемпотентность хранится в отдельной таблице `idempotency_keys`.
- При первом запросе в ней фиксируется ключ и хеш payload.
- Повтор с тем же ключом и тем же payload возвращает сохраненный `response_status` и `response_body`.
- Повтор с тем же ключом, но другим payload возвращает `422`.
- Списание баланса, создание `withdrawal`, запись в ledger и сохранение идемпотентного ответа происходят в одной транзакции.
- Схема БД накатывается приложением из embedded migrations, поэтому локальный запуск и тесты не зависят от внешнего `schema.sql`.

## API

### `POST /v1/withdrawals`

Пример запроса:

```json
{
  "user_id": "user-1",
  "amount": 100,
  "currency": "USDT",
  "destination": "wallet-abc",
  "idempotency_key": "withdraw-001"
}
```

Успех:

- `201 Created`

Ошибки:

- `400` при невалидном payload
- `401` при отсутствии или неверном Bearer token
- `404` если аккаунт пользователя не найден
- `409` при недостаточном балансе
- `422` если `idempotency_key` уже использован с другим payload

### `GET /v1/withdrawals/{id}`

Возвращает актуальное состояние вывода.

### `POST /v1/withdrawals/{id}/confirm`

Подтверждает вывод. Повторный confirm для уже подтвержденной заявки возвращает текущее состояние без ошибки.

### `GET /healthz`

Liveness probe без auth.

### `GET /readyz`

Readiness probe без auth. Проверяет доступность PostgreSQL через `Ping`.

## Пример вызова

```bash
curl -X POST http://localhost:18080/v1/withdrawals \
  -H 'Authorization: Bearer local-dev-token' \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"user-1","amount":100,"currency":"USDT","destination":"wallet-abc","idempotency_key":"withdraw-001"}'
```

## Dev workflow

```bash
make run
make test
make test-unit
make test-integration
make build
```

## Схема

- миграции лежат в [internal/platform/database/migrations/001_init.sql](/home/artem/code/go/test-task-rotmansstan/internal/platform/database/migrations/001_init.sql) и [internal/platform/database/migrations/002_seed.sql](/home/artem/code/go/test-task-rotmansstan/internal/platform/database/migrations/002_seed.sql)
- seed добавляет аккаунт `user-1` с балансом `1000 USDT`

## Тесты

```bash
GOCACHE=/tmp/go-build GOPROXY=off GOSUMDB=off go test ./...
```

Интеграционные тесты поднимают временный PostgreSQL через `docker run`.

## API contract

OpenAPI спецификация лежит в [api/openapi.yaml](/home/artem/code/go/test-task-rotmansstan/api/openapi.yaml).
