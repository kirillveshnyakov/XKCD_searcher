# XKCD Search Services

XKCD Search Services - учебный микросервисный проект на Go для поиска комиксов XKCD по ключевым словам. Пользователь отправляет обычный текстовый запрос, а сервис нормализует слова, сопоставляет их с сохраненными описаниями комиксов и возвращает наиболее подходящие результаты.

Проект показывает полный backend-сценарий: загрузку данных из внешнего API, обработку и хранение в Postgres, gRPC-взаимодействие между сервисами, REST API для внешних клиентов, авторизацию критичных операций, ограничение нагрузки, метрики и интеграционные тесты. Поиск реализован в двух вариантах: прямой поиск по базе данных и ускоренный поиск по периодически обновляемому in-memory индексу.

## Возможности

**Данные и поиск**

- загрузка, обновление и сброс базы комиксов XKCD;
- просмотр статистики и состояния базы данных;
- нормализация поисковых фраз через отдельный gRPC-сервис;
- поиск комиксов по словам из пользовательской фразы;
- индексированный поиск для ускорения частых запросов.

**API и безопасность**

- REST API для работы с поиском, состоянием базы и обновлением данных;
- JWT-авторизация для операций обновления и очистки базы;
- middleware для ограничения конкурентных запросов к `/api/search`;
- rate limit для регулирования нагрузки на `/api/isearch`.

**Эксплуатация и качество**

- экспорт HTTP-метрик в формате Prometheus;
- подсчет времени исполнения запросов по endpoint-ам и HTTP-статусам;
- готовое Docker Compose окружение с Postgres, VictoriaMetrics и Grafana;
- интеграционные тесты, которые проверяют весь кластер через публичный API.

## В разработке

- добавление брокера NATS для событийного обновления поискового индекса при изменениях в базе;
- unit-тесты для внутренней бизнес-логики сервисов и middleware;
- frontend-интерфейс для пользователей.

## Архитектура

Проект состоит из нескольких сервисов:

| Сервис | Назначение |
| --- | --- |
| `api` | REST API gateway, авторизация, middleware, метрики |
| `words` | gRPC-сервис нормализации слов и поисковых фраз |
| `update` | gRPC-сервис загрузки XKCD и обновления базы |
| `search` | gRPC-сервис поиска по базе и in-memory индексу |
| `postgres` | хранилище комиксов и нормализованных слов |
| `victoriametrics` | сбор и хранение метрик |
| `grafana` | визуализация метрик |

Внешний пользователь работает только с `api`. Внутри `api` обращается к `words`, `update` и `search` по gRPC.

## API

Все публичные endpoints доступны через `api` на `http://localhost:28080`.

| Метод | Endpoint | Назначение | Доступ |
| --- | --- | --- | --- |
| `GET` | `/api/ping` | проверка доступности `words`, `update`, `search` | публичный |
| `POST` | `/api/login` | выдача JWT-токена по логину и паролю | публичный |
| `POST` | `/api/db/update` | загрузка и обновление базы XKCD | `Authorization: Token <token>` |
| `DELETE` | `/api/db` | очистка базы | `Authorization: Token <token>` |
| `GET` | `/api/db/stats` | статистика базы: слова и комиксы | публичный |
| `GET` | `/api/db/status` | статус обновления: например `idle` или `running` | публичный |
| `GET` | `/api/search?phrase=<phrase>&limit=<limit>` | прямой поиск по базе | публичный |
| `GET` | `/api/isearch?phrase=<phrase>&limit=<limit>` | поиск по in-memory индексу | публичный |
| `GET` | `/metrics` | HTTP-метрики в формате Prometheus | публичный |

Для авторизации нужно получить токен:

```bash
TOKEN=$(curl -s -X POST \
  -d '{"name":"admin","password":"password"}' \
  http://localhost:28080/api/login)
```

И передавать его в защищенные endpoints:

```bash
curl -X POST -H "Authorization: Token $TOKEN" http://localhost:28080/api/db/update
curl -X DELETE -H "Authorization: Token $TOKEN" http://localhost:28080/api/db
```

Поиск принимает обязательный параметр `phrase` и необязательный `limit`; если `limit` не указан, используется `10`.

```bash
curl "http://localhost:28080/api/search?phrase=linux&limit=10"
curl "http://localhost:28080/api/isearch?phrase=linux&limit=10"
```

`/api/db/stats` возвращает количество загруженных комиксов и слов:

```json
{
  "words_total": 120000,
  "words_unique": 15000,
  "comics_fetched": 3000,
  "comics_total": 3000
}
```

`/metrics` экспортирует `http_request_duration_seconds` с метками `status` и `url`. Метрика считает время исполнения каждого HTTP-запроса, поэтому по ней можно отслеживать задержки по endpoint-ам и HTTP-статусам.

Индекс для `/api/isearch` обновляется периодически. Интервал задается переменной `INDEX_TTL`.

## Конфигурация

Сервисы читают настройки из `config.yaml` и переменных окружения. В Docker Compose основные значения уже заданы.

Важные переменные:

| Переменная | Назначение |
| --- | --- |
| `ADMIN_USER` | логин администратора |
| `ADMIN_PASSWORD` | пароль администратора |
| `TOKEN_TTL` | время жизни JWT-токена |
| `API_ADDRESS` | адрес REST API |
| `WORDS_ADDRESS` | адрес gRPC-сервиса `words` |
| `UPDATE_ADDRESS` | адрес gRPC-сервиса `update` |
| `SEARCH_ADDRESS` | адрес gRPC-сервиса `search` |
| `DB_ADDRESS` | строка подключения к Postgres |
| `XKCD_URL` | базовый URL XKCD |
| `XKCD_CONCURRENCY` | параллелизм загрузки XKCD |
| `SEARCH_CONCURRENCY` | лимит одновременных запросов к `/api/search` |
| `SEARCH_RATE` | rate limit для `/api/isearch` |
| `INDEX_TTL` | период обновления поискового индекса |

## Тестирование

Проверить компиляцию и package tests Go-модуля:

```bash
cd search-services
go test ./...
```

Unit-тесты для внутренних компонентов находятся в разработке.

Запустить полный интеграционный сценарий:

```bash
make test
```

Команда `make test` выполняет полный цикл:

1. удаляет старое compose-окружение и volumes;
2. собирает Docker images;
3. поднимает сервисы;
4. ждет запуска кластера;
5. запускает контейнер `tests`;
6. очищает окружение после тестов.

Интеграционные тесты проверяют:

- доступность всех сервисов через `/api/ping`;
- очистку и обновление базы XKCD;
- конкурентный запуск обновления;
- поиск по базе;
- индексированный поиск;
- обработку неверных query-параметров;
- авторизацию через `/api/login`;
- запрет доступа к критичным endpoints без токена;
- истечение JWT-токена;
- concurrency limit для `/api/search`;
- rate limit для `/api/isearch`.

## Мониторинг

VictoriaMetrics доступна на:

```text
http://localhost:8428
```

Grafana доступна на:

```text
http://localhost:3000
```

Пример dashboard-а с метриками:

![Grafana dashboard](metrics/grafana.png)

Dashboard можно импортировать из файла:

```text
metrics/dashboard.json
```

Конфигурация сбора метрик находится в:

```text
metrics/promscrape.yml
```

## Требования

- Go 1.25+
- Docker или Podman
- Make

## Быстрый старт

Установить локальные инструменты для разработки: `protolint`, `goimports`, `grpcurl`, генераторы protobuf, `bombardier` и `golangci-lint`:

```bash
make tools
```

Поднять Docker Compose окружение:

```bash
make up
```

Запустить интеграционные тесты против уже поднятого окружения:

```bash
make run-tests
```

Запустить полный цикл с очисткой, сборкой, стартом сервисов, тестами и финальной очисткой:

```bash
make test
```

Запустить линтеры:

```bash
make lint
```

Перегенерировать protobuf/gRPC-код:

```bash
make proto
```

Остановить окружение:

```bash
make down
```

Полностью очистить окружение вместе с volumes:

```bash
make clean
```
