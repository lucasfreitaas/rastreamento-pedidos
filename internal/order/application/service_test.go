package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"order-tracking/internal/order/application"
	"order-tracking/internal/order/domain"
	"order-tracking/internal/order/infra/memory"
)

func newService(repo domain.Repository) *application.Service {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	return application.NewService(repo, log).WithClock(func() time.Time { return now })
}

func createInput() application.CreateOrderInput {
	return application.CreateOrderInput{
		CustomerID: "customer-1",
		Items:      []application.ItemInput{{SKU: "KB-001", Quantity: 1, UnitPriceCents: 15000}},
	}
}

func TestService_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	svc := newService(memory.NewRepository())

	created, err := svc.CreateOrder(ctx, createInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := svc.GetOrder(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID || got.TotalCents != 15000 {
		t.Fatalf("unexpected order: %+v", got)
	}

	history, err := svc.StatusHistory(ctx, created.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 1 || history[0].To != domain.StatusCreated {
		t.Fatalf("expected initial CREATED entry, got %+v", history)
	}
}

func TestService_GetOrder_NotFound(t *testing.T) {
	svc := newService(memory.NewRepository())

	_, err := svc.GetOrder(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestService_ChangeStatus(t *testing.T) {
	ctx := context.Background()
	svc := newService(memory.NewRepository())
	order, err := svc.CreateOrder(ctx, createInput())
	if err != nil {
		t.Fatal(err)
	}

	updated, err := svc.ChangeStatus(ctx, order.ID, domain.StatusPaid)
	if err != nil {
		t.Fatalf("change status: %v", err)
	}
	if updated.Status != domain.StatusPaid {
		t.Fatalf("status = %s, want PAID", updated.Status)
	}

	_, err = svc.ChangeStatus(ctx, order.ID, domain.StatusDelivered)
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}

	history, _ := svc.StatusHistory(ctx, order.ID)
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
}

// failingRepo simula falha de infraestrutura para garantir que o erro é
// propagado com contexto (wrapping) e não engolido.
type failingRepo struct{ domain.Repository }

var errDBDown = errors.New("connection refused")

func (failingRepo) Create(context.Context, *domain.Order) error { return errDBDown }

func TestService_CreateOrder_WrapsRepositoryError(t *testing.T) {
	svc := newService(failingRepo{})

	_, err := svc.CreateOrder(context.Background(), createInput())
	if !errors.Is(err, errDBDown) {
		t.Fatalf("expected wrapped errDBDown, got %v", err)
	}
}

func TestService_ListOrders_RejectsUnknownStatus(t *testing.T) {
	svc := newService(memory.NewRepository())

	_, err := svc.ListOrders(context.Background(), domain.ListFilter{Status: "LOST"})
	if !errors.Is(err, domain.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}
