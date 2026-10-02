// Package orderhttp expõe os casos de uso de pedidos via REST.
package orderhttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"order-tracking/internal/order/application"
	"order-tracking/internal/order/domain"
	"order-tracking/internal/platform/httpx"
)

// OrderService é a interface que o handler consome. Declarar a interface
// aqui (no consumidor) facilita trocar a implementação nos testes.
type OrderService interface {
	CreateOrder(ctx context.Context, in application.CreateOrderInput) (*domain.Order, error)
	GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	ListOrders(ctx context.Context, f domain.ListFilter) ([]*domain.Order, error)
	ChangeStatus(ctx context.Context, id uuid.UUID, to domain.Status) (*domain.Order, error)
	StatusHistory(ctx context.Context, id uuid.UUID) ([]domain.StatusChange, error)
}

type Handler struct {
	svc OrderService
	log *slog.Logger
}

func NewHandler(svc *application.Service, log *slog.Logger) *Handler {
	return newHandler(svc, log)
}

func newHandler(svc OrderService, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /orders", h.create)
	mux.HandleFunc("GET /orders", h.list)
	mux.HandleFunc("GET /orders/{id}", h.get)
	mux.HandleFunc("PATCH /orders/{id}/status", h.changeStatus)
	mux.HandleFunc("GET /orders/{id}/history", h.history)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	order, err := h.svc.CreateOrder(r.Context(), req.toInput())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	w.Header().Set("Location", "/orders/"+order.ID.String())
	httpx.WriteJSON(w, http.StatusCreated, toOrderResponse(order))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	order, err := h.svc.GetOrder(r.Context(), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toOrderResponse(order))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := domain.ListFilter{Status: domain.Status(q.Get("status"))}

	var err error
	if filter.Limit, err = intParam(q.Get("limit")); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_query", "limit must be an integer")
		return
	}
	if filter.Offset, err = intParam(q.Get("offset")); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_query", "offset must be an integer")
		return
	}

	orders, err := h.svc.ListOrders(r.Context(), filter)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	resp := listOrdersResponse{Data: make([]orderResponse, 0, len(orders))}
	for _, o := range orders {
		resp.Data = append(resp.Data, toOrderResponse(o))
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) changeStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req changeStatusRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	order, err := h.svc.ChangeStatus(r.Context(), id, domain.Status(req.Status))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toOrderResponse(order))
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	history, err := h.svc.StatusHistory(r.Context(), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	resp := historyResponse{Data: make([]statusChangeResponse, 0, len(history))}
	for _, c := range history {
		resp.Data = append(resp.Data, toStatusChangeResponse(c))
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// writeError traduz erros de domínio em respostas HTTP. Erros não
// mapeados viram 500 e são logados, sem vazar detalhes internos ao cliente.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrOrderNotFound):
		httpx.WriteError(w, http.StatusNotFound, "order_not_found", "order not found")
	case errors.Is(err, domain.ErrInvalidOrder), errors.Is(err, domain.ErrInvalidStatus):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
	case errors.Is(err, domain.ErrInvalidTransition):
		httpx.WriteError(w, http.StatusConflict, "invalid_transition", err.Error())
	case errors.Is(err, domain.ErrConcurrentUpdate):
		httpx.WriteError(w, http.StatusConflict, "concurrent_update", "order was modified by another request, retry")
	default:
		h.log.ErrorContext(r.Context(), "unexpected error",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Any("error", err),
		)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID")
		return uuid.Nil, false
	}
	return id, true
}

func intParam(v string) (int, error) {
	if v == "" {
		return 0, nil
	}
	return strconv.Atoi(v)
}
