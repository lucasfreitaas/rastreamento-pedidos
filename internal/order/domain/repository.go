package domain

import (
	"context"

	"github.com/google/uuid"
)

// ListFilter define os filtros de listagem de pedidos.
type ListFilter struct {
	Status Status // vazio = todos
	Limit  int
	Offset int
}

// Repository é a porta de persistência do agregado Order.
// A interface fica no domínio; a implementação (Postgres) fica em infra.
type Repository interface {
	// Create persiste o pedido, seus itens e o evento inicial de status.
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id uuid.UUID) (*Order, error)
	List(ctx context.Context, filter ListFilter) ([]*Order, error)
	// SaveStatusChange atualiza o status somente se o pedido ainda estiver
	// em change.From; caso contrário retorna ErrConcurrentUpdate.
	SaveStatusChange(ctx context.Context, change StatusChange) error
	ListStatusHistory(ctx context.Context, orderID uuid.UUID) ([]StatusChange, error)
}
