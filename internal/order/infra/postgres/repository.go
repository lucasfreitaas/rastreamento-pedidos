// Package postgres implementa domain.Repository usando PostgreSQL (pgx).
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"order-tracking/internal/order/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

var _ domain.Repository = (*Repository)(nil)

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create grava pedido, itens e o primeiro registro de histórico na mesma
// transação: ou tudo é salvo, ou nada é.
func (r *Repository) Create(ctx context.Context, o *domain.Order) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO orders (id, customer_id, status, total_cents, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			o.ID, o.CustomerID, o.Status, o.TotalCents, o.CreatedAt, o.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert order: %w", err)
		}

		batch := &pgx.Batch{}
		for _, it := range o.Items {
			batch.Queue(`
				INSERT INTO order_items (order_id, sku, quantity, unit_price_cents)
				VALUES ($1, $2, $3, $4)`,
				o.ID, it.SKU, it.Quantity, it.UnitPriceCents,
			)
		}
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return fmt.Errorf("insert order items: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO order_status_history (order_id, from_status, to_status, occurred_at)
			VALUES ($1, NULL, $2, $3)`,
			o.ID, o.Status, o.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert initial status history: %w", err)
		}
		return nil
	})
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	var o domain.Order
	err := r.pool.QueryRow(ctx, `
		SELECT id, customer_id, status, total_cents, created_at, updated_at
		FROM orders WHERE id = $1`, id,
	).Scan(&o.ID, &o.CustomerID, &o.Status, &o.TotalCents, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select order: %w", err)
	}

	itemsByOrder, err := r.loadItems(ctx, []uuid.UUID{o.ID})
	if err != nil {
		return nil, err
	}
	o.Items = itemsByOrder[o.ID]
	return &o, nil
}

func (r *Repository) List(ctx context.Context, f domain.ListFilter) ([]*domain.Order, error) {
	// ($1 = '' OR status = $1) permite filtro opcional sem montar SQL dinâmico.
	rows, err := r.pool.Query(ctx, `
		SELECT id, customer_id, status, total_cents, created_at, updated_at
		FROM orders
		WHERE ($1 = '' OR status = $1)
		ORDER BY created_at DESC, id
		LIMIT $2 OFFSET $3`,
		string(f.Status), f.Limit, f.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("select orders: %w", err)
	}
	defer rows.Close()

	var (
		orders []*domain.Order
		ids    []uuid.UUID
	)
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.Status, &o.TotalCents, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, &o)
		ids = append(ids, o.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders: %w", err)
	}
	if len(orders) == 0 {
		return []*domain.Order{}, nil
	}

	// Uma única query para os itens de todos os pedidos (evita N+1).
	itemsByOrder, err := r.loadItems(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, o := range orders {
		o.Items = itemsByOrder[o.ID]
	}
	return orders, nil
}

func (r *Repository) loadItems(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID][]domain.Item, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT order_id, sku, quantity, unit_price_cents
		FROM order_items
		WHERE order_id = ANY($1)
		ORDER BY id`, orderIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("select order items: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]domain.Item, len(orderIDs))
	for rows.Next() {
		var (
			orderID uuid.UUID
			it      domain.Item
		)
		if err := rows.Scan(&orderID, &it.SKU, &it.Quantity, &it.UnitPriceCents); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		result[orderID] = append(result[orderID], it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order items: %w", err)
	}
	return result, nil
}

// SaveStatusChange usa controle de concorrência otimista: o UPDATE só
// acontece se o status no banco ainda for change.From.
func (r *Repository) SaveStatusChange(ctx context.Context, c domain.StatusChange) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE orders SET status = $1, updated_at = $2
			WHERE id = $3 AND status = $4`,
			c.To, c.OccurredAt, c.OrderID, c.From,
		)
		if err != nil {
			return fmt.Errorf("update order status: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrConcurrentUpdate
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO order_status_history (order_id, from_status, to_status, occurred_at)
			VALUES ($1, $2, $3, $4)`,
			c.OrderID, c.From, c.To, c.OccurredAt,
		)
		if err != nil {
			return fmt.Errorf("insert status history: %w", err)
		}
		return nil
	})
}

func (r *Repository) ListStatusHistory(ctx context.Context, orderID uuid.UUID) ([]domain.StatusChange, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT order_id, COALESCE(from_status, ''), to_status, occurred_at
		FROM order_status_history
		WHERE order_id = $1
		ORDER BY occurred_at, id`, orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("select status history: %w", err)
	}

	history, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.StatusChange, error) {
		var c domain.StatusChange
		err := row.Scan(&c.OrderID, &c.From, &c.To, &c.OccurredAt)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect status history: %w", err)
	}
	return history, nil
}
