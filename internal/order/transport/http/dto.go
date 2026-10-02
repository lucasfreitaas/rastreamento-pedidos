package orderhttp

import (
	"time"

	"order-tracking/internal/order/application"
	"order-tracking/internal/order/domain"
)

// DTOs separam o contrato da API do modelo de domínio: podemos mudar o
// domínio sem quebrar clientes, e vice-versa.

type createOrderRequest struct {
	CustomerID string        `json:"customer_id"`
	Items      []itemRequest `json:"items"`
}

type itemRequest struct {
	SKU            string `json:"sku"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

func (r createOrderRequest) toInput() application.CreateOrderInput {
	in := application.CreateOrderInput{CustomerID: r.CustomerID}
	for _, it := range r.Items {
		in.Items = append(in.Items, application.ItemInput{
			SKU:            it.SKU,
			Quantity:       it.Quantity,
			UnitPriceCents: it.UnitPriceCents,
		})
	}
	return in
}

type changeStatusRequest struct {
	Status string `json:"status"`
}

type orderResponse struct {
	ID           string         `json:"id"`
	CustomerID   string         `json:"customer_id"`
	Status       string         `json:"status"`
	NextStatuses []string       `json:"next_statuses"`
	TotalCents   int64          `json:"total_cents"`
	Items        []itemResponse `json:"items"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type itemResponse struct {
	SKU            string `json:"sku"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

type listOrdersResponse struct {
	Data []orderResponse `json:"data"`
}

type statusChangeResponse struct {
	From       *string   `json:"from"`
	To         string    `json:"to"`
	OccurredAt time.Time `json:"occurred_at"`
}

type historyResponse struct {
	Data []statusChangeResponse `json:"data"`
}

func toOrderResponse(o *domain.Order) orderResponse {
	items := make([]itemResponse, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, itemResponse{
			SKU:            it.SKU,
			Quantity:       it.Quantity,
			UnitPriceCents: it.UnitPriceCents,
		})
	}
	next := make([]string, 0)
	for _, s := range o.Status.NextStatuses() {
		next = append(next, string(s))
	}
	return orderResponse{
		ID:           o.ID.String(),
		CustomerID:   o.CustomerID,
		Status:       string(o.Status),
		NextStatuses: next,
		TotalCents:   o.TotalCents,
		Items:        items,
		CreatedAt:    o.CreatedAt.UTC(),
		UpdatedAt:    o.UpdatedAt.UTC(),
	}
}

func toStatusChangeResponse(c domain.StatusChange) statusChangeResponse {
	resp := statusChangeResponse{To: string(c.To), OccurredAt: c.OccurredAt.UTC()}
	if c.From != "" {
		from := string(c.From)
		resp.From = &from
	}
	return resp
}
