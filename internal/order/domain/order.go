// Package domain contém as regras de negócio de pedidos, sem dependência
// de banco de dados, HTTP ou qualquer detalhe de infraestrutura.
package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Item representa uma linha do pedido.
type Item struct {
	SKU            string
	Quantity       int
	UnitPriceCents int64
}

// SubtotalCents retorna quantidade * preço unitário.
func (i Item) SubtotalCents() int64 {
	return int64(i.Quantity) * i.UnitPriceCents
}

func (i Item) validate() error {
	if strings.TrimSpace(i.SKU) == "" {
		return fmt.Errorf("%w: item sku is required", ErrInvalidOrder)
	}
	if i.Quantity <= 0 {
		return fmt.Errorf("%w: item %q quantity must be greater than zero", ErrInvalidOrder, i.SKU)
	}
	if i.UnitPriceCents < 0 {
		return fmt.Errorf("%w: item %q unit price cannot be negative", ErrInvalidOrder, i.SKU)
	}
	return nil
}

// Order é o agregado raiz. Toda mudança de estado passa pelos seus métodos,
// garantindo que as regras de transição sejam sempre respeitadas.
type Order struct {
	ID         uuid.UUID
	CustomerID string
	Items      []Item
	Status     Status
	TotalCents int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewOrder valida os dados de entrada e cria um pedido no status inicial.
func NewOrder(customerID string, items []Item, now time.Time) (*Order, error) {
	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return nil, fmt.Errorf("%w: customer_id is required", ErrInvalidOrder)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: at least one item is required", ErrInvalidOrder)
	}

	var total int64
	copied := make([]Item, 0, len(items))
	for _, it := range items {
		if err := it.validate(); err != nil {
			return nil, err
		}
		total += it.SubtotalCents()
		copied = append(copied, it)
	}

	now = normalizeTime(now)
	return &Order{
		ID:         uuid.New(),
		CustomerID: customerID,
		Items:      copied,
		Status:     StatusCreated,
		TotalCents: total,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// ChangeStatus aplica uma transição de status, se ela for permitida.
// Retorna o evento gerado para que a camada de aplicação possa persisti-lo
// e, futuramente, publicá-lo (SNS).
func (o *Order) ChangeStatus(to Status, now time.Time) (StatusChange, error) {
	if !to.IsValid() {
		return StatusChange{}, fmt.Errorf("%w: unknown status %q", ErrInvalidStatus, to)
	}
	if !o.Status.CanTransitionTo(to) {
		return StatusChange{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, o.Status, to)
	}

	change := StatusChange{
		OrderID:    o.ID,
		From:       o.Status,
		To:         to,
		OccurredAt: normalizeTime(now),
	}
	o.Status = to
	o.UpdatedAt = change.OccurredAt
	return change, nil
}

// StatusChange registra uma mudança de status (histórico do pedido).
type StatusChange struct {
	OrderID    uuid.UUID
	From       Status // vazio na criação do pedido
	To         Status
	OccurredAt time.Time
}

// normalizeTime usa UTC e precisão de microssegundos, a mesma do
// TIMESTAMPTZ do PostgreSQL. Sem isso, o valor em memória (nanossegundos)
// difere do valor relido do banco.
func normalizeTime(t time.Time) time.Time {
	return t.UTC().Truncate(time.Microsecond)
}
