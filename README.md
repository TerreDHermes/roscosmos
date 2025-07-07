## Первый этап - OrderService
### Что это за сервис
Сервис заказов управляет заказами на постройку космических кораблей. Все заказы хранятся во внутренней `map`-структуре в
памяти. Сервис взаимодействует с `InventoryService` для расчёта цены заказа и с `PaymentService` для проведения оплаты.
### Архитектура 
```
├── Taskfile.yml
├── buf.work.yaml
├── go.work
├── go.work.sum
├── package-lock.json
├── package.json
├── order
│   ├── cmd
│   │   └── main.go
│   ├── go.mod
│   └── go.sum
```
📁 Все контракты находятся в папке [`/shared`](shared). OpenAPI-контракты — в `shared/api`. Сгенерированные файлы будут находиться в `shared/pkg/openapi`.
```
└── shared
    ├── api
    │   └── order
    │       └── v1
    │           ├── components
    │           │   ├── create_order_request.yaml
    │           │   ├── create_order_response.yaml
    │           │   ├── enums
    │           │   │   ├── order_status.yaml
    │           │   │   └── payment_method.yaml
    │           │   ├── errors
    │           │   │   ├── bad_gateway_error.yaml
    │           │   │   ├── bad_request_error.yaml
    │           │   │   ├── conflict_error.yaml
    │           │   │   ├── forbidden_error.yaml
    │           │   │   ├── generic_error.yaml
    │           │   │   ├── internal_server_error.yaml
    │           │   │   ├── not_found_error.yaml
    │           │   │   ├── rate_limit_error.yaml
    │           │   │   ├── service_unavailable_error.yaml
    │           │   │   ├── unauthorized_error.yaml
    │           │   │   └── validation_error.yaml
    │           │   ├── get_order_response.yaml
    │           │   ├── order_dto.yaml
    │           │   ├── pay_order_request.yaml
    │           │   └── pay_order_response.yaml
    │           ├── order.openapi.yaml
    │           ├── params
    │           │   └── order_uuid.yaml
    │           └── paths
    │               ├── order_by_uuid.yaml
    │               ├── order_cancel.yaml
    │               ├── order_pay.yaml
    │               └── orders.yaml
    ├── go.mod
    ├── go.sum
    ├── pkg
    │   ├── openapi
    │   │   └── order
    │   │       └── v1
    │   │           ├── oas_cfg_gen.go
    │   │           ├── oas_client_gen.go
    │   │           ├── oas_handlers_gen.go
    │   │           ├── oas_interfaces_gen.go
    │   │           ├── oas_json_gen.go
    │   │           ├── oas_labeler_gen.go
    │   │           ├── oas_middleware_gen.go
    │   │           ├── oas_operations_gen.go
    │   │           ├── oas_parameters_gen.go
    │   │           ├── oas_request_decoders_gen.go
    │   │           ├── oas_request_encoders_gen.go
    │   │           ├── oas_response_decoders_gen.go
    │   │           ├── oas_response_encoders_gen.go
    │   │           ├── oas_router_gen.go
    │   │           ├── oas_schemas_gen.go
    │   │           ├── oas_server_gen.go
    │   │           ├── oas_unimplemented_gen.go
    │   │           └── oas_validators_gen.go
```
### Что нужно сделать
Реализовать **HTTP API для `OrderService`**, строго следуя OpenAPI-контракту [`order_service_contracts.md`](contracts/order_service_contracts.md).

### 📌 Цели

- Освоить интеграцию нескольких gRPC/HTTP-сервисов
- Управлять внутренним состоянием приложения (in-memory)
- Реализовать жизненный цикл заказа: создание → оплата → отмена

### Методы для реализации

#### `POST /api/v1/orders` — создание заказа
#### `POST /api/v1/orders/{order_uuid}/pay` — оплата заказа
#### `GET /api/v1/orders/{order_uuid}` — получить заказ по UUID
#### `POST /api/v1/orders/{order_uuid}/cancel` — отменить заказ

### ✅ Критерии приёмки

- Все 4 метода реализованы строго по OpenAPI-контракту
- Обработка статусов заказа реализована корректно
- Заказы корректно сохраняются и обновляются в `map` и хранилище защищено от гонок данных
- Валидация запросов и обработка ошибок присутствует

### 🔍 Подсказки

- UUID можно сгенерировать с помощью `github.com/google/uuid`
- Статусы и методы оплаты лучше оформить через enum-константы

