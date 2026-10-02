-- +goose Up
CREATE TABLE orders (
    id          UUID        PRIMARY KEY,
    customer_id TEXT        NOT NULL,
    status      TEXT        NOT NULL,
    total_cents BIGINT      NOT NULL CHECK (total_cents >= 0),
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT orders_status_check CHECK (
        status IN ('CREATED', 'PAID', 'PICKING', 'SHIPPED', 'DELIVERED', 'CANCELED')
    )
);

CREATE TABLE order_items (
    id               BIGSERIAL PRIMARY KEY,
    order_id         UUID      NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    sku              TEXT      NOT NULL,
    quantity         INT       NOT NULL CHECK (quantity > 0),
    unit_price_cents BIGINT    NOT NULL CHECK (unit_price_cents >= 0)
);

CREATE INDEX idx_order_items_order_id ON order_items (order_id);

-- +goose Down
DROP TABLE order_items;
DROP TABLE orders;
