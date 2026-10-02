//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"order-tracking/internal/order/domain"
	"order-tracking/internal/order/infra/postgres"
	"order-tracking/internal/platform/database"
)

// Rodar com:
//
//	TEST_DATABASE_URL=postgres://app:app@localhost:5432/orders_test?sslmode=disable \
//	  go test -tags=integration ./internal/order/infra/postgres/...
func setup(t *testing.T) *postgres.Repository {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := database.Migrate(ctx, pool, log); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE orders CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return postgres.NewRepository(pool)
}

func newOrder(t *testing.T) *domain.Order {
	t.Helper()
	o, err := domain.NewOrder("customer-1", []domain.Item{
		{SKU: "KB-001", Quantity: 2, UnitPriceCents: 15000},
		{SKU: "MS-002", Quantity: 1, UnitPriceCents: 8990},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestRepository_CreateAndGet(t *testing.T) {
	repo := setup(t)
	ctx := context.Background()
	o := newOrder(t)

	if err := repo.Create(ctx, o); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(ctx, o.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TotalCents != o.TotalCents || len(got.Items) != 2 || got.Status != domain.StatusCreated {
		t.Fatalf("unexpected order: %+v", got)
	}
}

func TestRepository_StatusChangeAndHistory(t *testing.T) {
	repo := setup(t)
	ctx := context.Background()
	o := newOrder(t)
	if err := repo.Create(ctx, o); err != nil {
		t.Fatal(err)
	}

	change, err := o.ChangeStatus(domain.StatusPaid, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveStatusChange(ctx, change); err != nil {
		t.Fatalf("save change: %v", err)
	}

	// Repetir a mesma mudança simula outro processo que leu o pedido
	// antes da atualização: deve falhar com ErrConcurrentUpdate.
	if err := repo.SaveStatusChange(ctx, change); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("expected ErrConcurrentUpdate, got %v", err)
	}

	history, err := repo.ListStatusHistory(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].From != "" || history[1].To != domain.StatusPaid {
		t.Fatalf("unexpected history: %+v", history)
	}
}

func TestRepository_ListFiltersByStatus(t *testing.T) {
	repo := setup(t)
	ctx := context.Background()

	paid := newOrder(t)
	created := newOrder(t)
	for _, o := range []*domain.Order{paid, created} {
		if err := repo.Create(ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	change, _ := paid.ChangeStatus(domain.StatusPaid, time.Now())
	if err := repo.SaveStatusChange(ctx, change); err != nil {
		t.Fatal(err)
	}

	list, err := repo.List(ctx, domain.ListFilter{Status: domain.StatusPaid, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != paid.ID || len(list[0].Items) != 2 {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	repo := setup(t)
	_, err := repo.GetByID(context.Background(), newOrder(t).ID)
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}
}
