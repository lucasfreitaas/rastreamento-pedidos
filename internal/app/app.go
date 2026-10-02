// Package app compõe todos os módulos Fx da order-api.
package app

import (
	"go.uber.org/fx"

	"order-tracking/internal/order"
	"order-tracking/internal/platform/config"
	"order-tracking/internal/platform/database"
	"order-tracking/internal/platform/httpserver"
	"order-tracking/internal/platform/logger"
	"order-tracking/internal/web"
)

func Options() fx.Option {
	return fx.Options(
		config.Module,
		logger.Module,
		fx.WithLogger(logger.FxLogger),
		database.Module,
		httpserver.Module,
		fx.Provide(
			httpserver.AsRoute(httpserver.NewHealthHandler),
			httpserver.AsRoute(web.NewHandler),
		),
		order.Module,
	)
}
