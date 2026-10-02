package domain_test

import (
	"errors"
	"testing"
	"time"

	"order-tracking/internal/order/domain"
)

var fixedNow = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

func validItems() []domain.Item {
	return []domain.Item{
		{SKU: "KB-001", Quantity: 2, UnitPriceCents: 15000},
		{SKU: "MS-002", Quantity: 1, UnitPriceCents: 8990},
	}
}

func TestNewOrder(t *testing.T) {
	tests := []struct {
		name       string
		customerID string
		items      []domain.Item
		wantErr    error
		wantTotal  int64
	}{
		{
			name:       "valid order calculates total",
			customerID: "customer-1",
			items:      validItems(),
			wantTotal:  2*15000 + 8990,
		},
		{
			name:       "empty customer",
			customerID: "   ",
			items:      validItems(),
			wantErr:    domain.ErrInvalidOrder,
		},
		{
			name:       "no items",
			customerID: "customer-1",
			wantErr:    domain.ErrInvalidOrder,
		},
		{
			name:       "zero quantity",
			customerID: "customer-1",
			items:      []domain.Item{{SKU: "A", Quantity: 0, UnitPriceCents: 100}},
			wantErr:    domain.ErrInvalidOrder,
		},
		{
			name:       "negative price",
			customerID: "customer-1",
			items:      []domain.Item{{SKU: "A", Quantity: 1, UnitPriceCents: -1}},
			wantErr:    domain.ErrInvalidOrder,
		},
		{
			name:       "missing sku",
			customerID: "customer-1",
			items:      []domain.Item{{Quantity: 1, UnitPriceCents: 100}},
			wantErr:    domain.ErrInvalidOrder,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := domain.NewOrder(tt.customerID, tt.items, fixedNow)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if order.Status != domain.StatusCreated {
				t.Errorf("status = %s, want %s", order.Status, domain.StatusCreated)
			}
			if order.TotalCents != tt.wantTotal {
				t.Errorf("total = %d, want %d", order.TotalCents, tt.wantTotal)
			}
			if !order.CreatedAt.Equal(fixedNow) {
				t.Errorf("created_at = %v, want %v", order.CreatedAt, fixedNow)
			}
		})
	}
}

func TestNewOrder_CopiesItems(t *testing.T) {
	items := validItems()
	order, err := domain.NewOrder("customer-1", items, fixedNow)
	if err != nil {
		t.Fatal(err)
	}

	items[0].Quantity = 999 // alterar o slice original não pode afetar o pedido
	if order.Items[0].Quantity == 999 {
		t.Fatal("order must not share the items slice with the caller")
	}
}

func TestOrder_ChangeStatus(t *testing.T) {
	tests := []struct {
		name    string
		path    []domain.Status // transições aplicadas em sequência
		wantErr error
	}{
		{name: "happy path until delivered", path: []domain.Status{
			domain.StatusPaid, domain.StatusPicking, domain.StatusShipped, domain.StatusDelivered,
		}},
		{name: "cancel before payment", path: []domain.Status{domain.StatusCanceled}},
		{name: "cancel while picking", path: []domain.Status{
			domain.StatusPaid, domain.StatusPicking, domain.StatusCanceled,
		}},
		{name: "cannot skip payment", path: []domain.Status{domain.StatusShipped},
			wantErr: domain.ErrInvalidTransition},
		{name: "cannot cancel after shipped", path: []domain.Status{
			domain.StatusPaid, domain.StatusPicking, domain.StatusShipped, domain.StatusCanceled,
		}, wantErr: domain.ErrInvalidTransition},
		{name: "cannot leave final status", path: []domain.Status{
			domain.StatusCanceled, domain.StatusPaid,
		}, wantErr: domain.ErrInvalidTransition},
		{name: "unknown status", path: []domain.Status{"LOST"},
			wantErr: domain.ErrInvalidStatus},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := domain.NewOrder("customer-1", validItems(), fixedNow)
			if err != nil {
				t.Fatal(err)
			}

			var lastErr error
			for i, to := range tt.path {
				from := order.Status
				at := fixedNow.Add(time.Duration(i+1) * time.Minute)
				change, err := order.ChangeStatus(to, at)
				if err != nil {
					lastErr = err
					if order.Status != from {
						t.Fatalf("status changed even though transition failed")
					}
					break
				}
				if change.From != from || change.To != to {
					t.Errorf("change = %s->%s, want %s->%s", change.From, change.To, from, to)
				}
				if !order.UpdatedAt.Equal(at) {
					t.Errorf("updated_at not refreshed")
				}
			}

			if tt.wantErr == nil && lastErr != nil {
				t.Fatalf("unexpected error: %v", lastErr)
			}
			if tt.wantErr != nil && !errors.Is(lastErr, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, lastErr)
			}
		})
	}
}
