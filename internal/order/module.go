// Package order agrupa o módulo Fx do contexto de pedidos.
package order

import (
	"go.uber.org/fx"

	"order-tracking/internal/order/application"
	"order-tracking/internal/order/domain"
	"order-tracking/internal/order/infra/postgres"
	orderhttp "order-tracking/internal/order/transport/http"
	"order-tracking/internal/platform/httpserver"
)

var Module = fx.Module("order",
	fx.Provide(
		// O serviço depende da interface domain.Repository; aqui dizemos
		// ao Fx que a implementação Postgres satisfaz essa interface.
		fx.Annotate(postgres.NewRepository, fx.As(new(domain.Repository))),
		application.NewService,
		httpserver.AsRoute(orderhttp.NewHandler),
	),
)
