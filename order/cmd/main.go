package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	orderV1 "github.com/TerreDHermes/roscosmos/shared/pkg/openapi/order/v1"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	httpPort = "8080"
	// Таймауты для HTTP-сервера
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

var ErrOrderNotFound = errors.New("order not found")

// OrderStorage представляет потокобезопасное хранилище данных о заказах
type OrderStorage struct {
	mu     sync.RWMutex
	orders map[string]*orderV1.OrderDto
}

// NewOrderStorage создает новое хранилище данных о заказах
func NewOrderStorage() *OrderStorage {
	return &OrderStorage{
		orders: make(map[string]*orderV1.OrderDto),
	}
}

func (ost *OrderStorage) CreateOrderInStorage(orderUUID string, totalPrice float32, userUUID string, partUUIDs []string) (*orderV1.OrderDto, error) {
	ost.mu.Lock()
	defer ost.mu.Unlock()

	ost.orders[orderUUID] = &orderV1.OrderDto{}
	order_status := orderV1.NewOptOrderStatus(orderV1.OrderStatusPENDINGPAYMENT)

	ost.orders[orderUUID].SetOrderUUID(orderUUID)
	ost.orders[orderUUID].SetUserUUID(userUUID)
	ost.orders[orderUUID].SetPartUuids(partUUIDs)
	ost.orders[orderUUID].SetTotalPrice(totalPrice)
	ost.orders[orderUUID].SetStatus(order_status)

	ost.orders[orderUUID].PaymentMethod.SetTo("")

	return ost.orders[orderUUID], nil
}

func (ost *OrderStorage) GetOrderFromStorage(orderUUID string) (*orderV1.OrderDto, error) {
	ost.mu.RLock()
	defer ost.mu.RUnlock()
	if _, ok := ost.orders[orderUUID]; !ok {
		return nil, ErrOrderNotFound
	}
	return ost.orders[orderUUID], nil
}

// статус → `PAID`, сохраняет `transaction_uuid`, `payment_method`.
func (ost *OrderStorage) SetPayOrderInformation(orderUUID string, status orderV1.OrderStatus, transactionUUID string, paymentMethod orderV1.PaymentMethod) error {
	ost.mu.RLock()
	defer ost.mu.RUnlock()
	paymentMethodOpt := orderV1.NewOptPaymentMethod(paymentMethod)
	orderStatusOpt := orderV1.NewOptOrderStatus(status)

	ost.orders[orderUUID].SetStatus(orderStatusOpt)
	ost.orders[orderUUID].SetTransactionUUID(transactionUUID)
	ost.orders[orderUUID].SetPaymentMethod(paymentMethodOpt)
	return nil
}

// OrderHandler реализует интерфейс orderV1.Handler для обработки запросов к API заказов
type OrderHandler struct {
	storage *OrderStorage
}

// NewOrderHandler создает новый обработчик запросов к API заказов
func NewOrderHandler(storage *OrderStorage) *OrderHandler {
	return &OrderHandler{
		storage: storage,
	}
}

// Получаем детали через `InventoryService.ListParts` и считаем сумму (total_price)
func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderV1.CreateOrderRequest) (orderV1.CreateOrderRes, error) {
	orderUUID := uuid.New()
	totalPrice := 100.00

	orderDto, _ := h.storage.CreateOrderInStorage(orderUUID.String(), float32(totalPrice), req.UserUUID, req.PartUuids)

	return &orderV1.CreateOrderResponse{
		OrderUUID:  orderDto.GetOrderUUID(),
		TotalPrice: orderDto.GetTotalPrice(),
	}, nil
}

// - Ищет заказ по UUID. Если найден — возвращает. Если не найден — 404 Not Found.
func (h *OrderHandler) GetOrder(ctx context.Context, params orderV1.GetOrderParams) (orderV1.GetOrderRes, error) {
	orderDto, err := h.storage.GetOrderFromStorage(params.OrderUUID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			return &orderV1.NotFoundError{Message: "Order not found"}, nil
		}
		return nil, err
	}

	return &orderV1.GetOrderResponse{
		OrderDto: *orderDto,
	}, nil
}

// - Находит заказ по `order_uuid`. Если не существует — возвращает 404 Not Found.
// - Вызывает `PaymentService.PayOrder`, передаёт `user_uuid`, `order_uuid` и `payment_method`. Получает`transaction_uuid`.
// - Обновляет заказ: статус → `PAID`, сохраняет `transaction_uuid`, `payment_method`.
func (h *OrderHandler) PayOrder(ctx context.Context, req *orderV1.PayOrderRequest, params orderV1.PayOrderParams) (orderV1.PayOrderRes, error) {
	orderDto, err := h.storage.GetOrderFromStorage(params.OrderUUID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			return &orderV1.NotFoundError{Message: "Order not found"}, nil
		}
		return nil, err
	}

	err = ValidateStatus(string(orderDto.GetStatus().Value))
	if err != nil {
		return &orderV1.BadRequestError{
			Message: err.Error(),
		}, nil
	}

	paymentMethod, err := ValidatePaymentMethod(req.GetPaymentMethod())
	if err != nil {
		return &orderV1.BadRequestError{
			Message: err.Error(),
		}, nil
	}

	transactionUUID := uuid.New()

	h.storage.SetPayOrderInformation(orderDto.OrderUUID, orderV1.OrderStatusPAID, transactionUUID.String(), paymentMethod)

	return &orderV1.PayOrderResponse{
		TransactionUUID: transactionUUID.String(),
	}, nil
}

func (h *OrderHandler) OrderCancel(ctx context.Context, params orderV1.OrderCancelParams) (orderV1.OrderCancelRes, error) {
	return nil, nil
}

func (h *OrderHandler) NewError(ctx context.Context, err error) *orderV1.GenericErrorStatusCode {
	return &orderV1.GenericErrorStatusCode{}
}

func ValidatePaymentMethod(pm string) (orderV1.PaymentMethod, error) {
	paymentMethod := orderV1.PaymentMethod(pm)
	switch paymentMethod {
	case orderV1.PaymentMethodCARD, orderV1.PaymentMethodSBP, orderV1.PaymentMethodCREDITCARD, orderV1.PaymentMethodINVESTORMONEY, orderV1.PaymentMethodUNKNOWN:
		return paymentMethod, nil
	default:
		return "", fmt.Errorf("invalid payment method: %s", pm)
	}
}

func ValidateStatus(status string) error {
	switch status {
	case string(orderV1.OrderStatusPENDINGPAYMENT):
		return nil
	default:
		return fmt.Errorf("invalid status: %s", status)
	}
}

func main() {
	storage := NewOrderStorage()
	handler := NewOrderHandler(storage)

	orderServer, err := orderV1.NewServer(handler)
	if err != nil {
		log.Fatalf("Не удалось создать сервер OpenAPI: %v", err)
	}

	// Инициализируем роутер Chi
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	// Монтируем обработчики OpenAPI
	r.Mount("/", orderServer)

	// Запускаем HTTP-сервер
	server := &http.Server{
		Addr:              net.JoinHostPort("localhost", httpPort),
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout, // Защита от Slowloris атак - тип DDoS-атаки, при которой
		// атакующий умышленно медленно отправляет HTTP-заголовки, удерживая соединения открытыми и истощая
		// пул доступных соединений на сервере. ReadHeaderTimeout принудительно закрывает соединение,
		// если клиент не успел отправить все заголовки за отведенное время.
	}

	// Запускаем сервер в отдельной горутине
	go func() {
		log.Printf("🚀 HTTP-сервер запущен на порту %s\n", httpPort)
		err = server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("❌ Ошибка запуска сервера: %v\n", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Завершение работы сервера...")

	// Создаем контекст с таймаутом для остановки сервера
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	err = server.Shutdown(ctx)
	if err != nil {
		log.Printf("❌ Ошибка при остановке сервера: %v\n", err)
	}

	log.Println("✅ Сервер остановлен")
}
