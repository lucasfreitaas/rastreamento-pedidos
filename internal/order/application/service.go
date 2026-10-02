// Package application orquestra os casos de uso de pedidos: recebe a
// intenção (criar, mudar status...), aplica as regras do domínio e
// persiste o resultado pelo Repository.
package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"order-tracking/internal/order/domain"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

// Clock permite controlar o tempo nos testes.
type Clock func() time.Time

type ItemInput struct {
	SKU            string
	Quantity       int
	UnitPriceCents int64
}

type CreateOrderInput struct {
	CustomerID string
	Items      []ItemInput
}

type Service struct {
	repo domain.Repository
	now  Clock
	log  *slog.Logger
}

func NewService(repo domain.Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, now: time.Now, log: log}
}

// WithClock é usado nos testes para fixar o horário.
func (s *Service) WithClock(c Clock) *Service {
	s.now = c
	return s
}

func (s *Service) CreateOrder(ctx context.Context, in CreateOrderInput) (*domain.Order, error) {
	items := make([]domain.Item, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, domain.Item{
			SKU:            it.SKU,
			Quantity:       it.Quantity,
			UnitPriceCents: it.UnitPriceCents,
		})
	}

	order, err := domain.NewOrder(in.CustomerID, items, s.now())
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	s.log.InfoContext(ctx, "order created",
		slog.String("order_id", order.ID.String()),
		slog.Int64("total_cents", order.TotalCents),
	)
	return order, nil
}

func (s *Service) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get order %s: %w", id, err)
	}
	return order, nil
}

func (s *Service) ListOrders(ctx context.Context, filter domain.ListFilter) ([]*domain.Order, error) {
	if filter.Status != "" && !filter.Status.IsValid() {
		return nil, fmt.Errorf("%w: unknown status %q", domain.ErrInvalidStatus, filter.Status)
	}
	if filter.Limit <= 0 {
		filter.Limit = defaultListLimit
	}
	if filter.Limit > maxListLimit {
		filter.Limit = maxListLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	orders, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	return orders, nil
}

// ChangeStatus carrega o pedido, aplica a transição no domínio e persiste.
// Se outro processo mudar o status no meio do caminho, o repositório
// devolve domain.ErrConcurrentUpdate e nada é sobrescrito.
func (s *Service) ChangeStatus(ctx context.Context, id uuid.UUID, to domain.Status) (*domain.Order, error) {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get order %s: %w", id, err)
	}

	change, err := order.ChangeStatus(to, s.now())
	if err != nil {
		return nil, err
	}

	if err := s.repo.SaveStatusChange(ctx, change); err != nil {
		return nil, fmt.Errorf("save status change: %w", err)
	}

	s.log.InfoContext(ctx, "order status changed",
		slog.String("order_id", id.String()),
		slog.String("from", string(change.From)),
		slog.String("to", string(change.To)),
	)
	return order, nil
}

func (s *Service) StatusHistory(ctx context.Context, id uuid.UUID) ([]domain.StatusChange, error) {
	// Garante 404 quando o pedido não existe, em vez de lista vazia.
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, fmt.Errorf("get order %s: %w", id, err)
	}
	history, err := s.repo.ListStatusHistory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list status history: %w", err)
	}
	return history, nil
}
