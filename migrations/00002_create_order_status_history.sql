-- +goose Up
-- Histórico de status: base para auditoria e, depois, para os eventos
-- publicados no SNS.
CREATE TABLE order_status_history (
    id          BIGSERIAL   PRIMARY KEY,
    order_id    UUID        NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    from_status TEXT        NULL,
    to_status   TEXT        NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_order_status_history_order_id
    ON order_status_history (order_id, occurred_at);

-- A listagem filtra por status e ordena pelos mais recentes.
CREATE INDEX idx_orders_status_created_at
    ON orders (status, created_at DESC);

-- +goose Down
DROP INDEX idx_orders_status_created_at;
DROP TABLE order_status_history;
