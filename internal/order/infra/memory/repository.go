// Package memory implementa domain.Repository em memória. É usada nos
// testes das camadas de aplicação e HTTP, sem precisar de banco.
package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/google/uuid"

	"order-tracking/internal/order/domain"
)

type Repository struct {
	mu      sync.RWMutex
	orders  map[uuid.UUID]domain.Order
	history map[uuid.UUID][]domain.StatusChange
}

var _ domain.Repository = (*Repository)(nil)

func NewRepository() *Repository {
	return &Repository{
		orders:  make(map[uuid.UUID]domain.Order),
		history: make(map[uuid.UUID][]domain.StatusChange),
	}
}

// clone evita que quem chama altere o estado interno por referência.
func clone(o domain.Order) *domain.Order {
	o.Items = append([]domain.Item(nil), o.Items...)
	return &o
}

func (r *Repository) Create(_ context.Context, o *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[o.ID] = *clone(*o)
	r.history[o.ID] = []domain.StatusChange{{OrderID: o.ID, To: o.Status, OccurredAt: o.CreatedAt}}
	return nil
}

func (r *Repository) GetByID(_ context.Context, id uuid.UUID) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return clone(o), nil
}

func (r *Repository) List(_ context.Context, f domain.ListFilter) ([]*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domain.Order, 0)
	for _, o := range r.orders {
		if f.Status == "" || o.Status == f.Status {
			result = append(result, clone(o))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })

	if f.Offset >= len(result) {
		return []*domain.Order{}, nil
	}
	result = result[f.Offset:]
	if f.Limit > 0 && f.Limit < len(result) {
		result = result[:f.Limit]
	}
	return result, nil
}

func (r *Repository) SaveStatusChange(_ context.Context, c domain.StatusChange) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.orders[c.OrderID]
	if !ok {
		return domain.ErrOrderNotFound
	}
	if o.Status != c.From {
		return domain.ErrConcurrentUpdate
	}
	o.Status = c.To
	o.UpdatedAt = c.OccurredAt
	r.orders[c.OrderID] = o
	r.history[c.OrderID] = append(r.history[c.OrderID], c)
	return nil
}

func (r *Repository) ListStatusHistory(_ context.Context, id uuid.UUID) ([]domain.StatusChange, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]domain.StatusChange(nil), r.history[id]...), nil
}
