// Package httpserver monta o servidor HTTP. Cada módulo de negócio
// registra suas rotas implementando RouteRegistrar e se adicionando ao
// grupo "routes" do Fx, sem que o servidor precise conhecê-los.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"go.uber.org/fx"

	"order-tracking/internal/platform/config"
)

var Module = fx.Module("httpserver",
	fx.Provide(
		fx.Annotate(NewMux, fx.ParamTags(`group:"routes"`)),
		NewServer,
	),
	fx.Invoke(func(*http.Server) {}), // força a criação do servidor
)

// RouteRegistrar é implementado por quem expõe rotas HTTP.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

// AsRoute anota um construtor para que seu resultado entre no grupo de rotas.
func AsRoute(constructor any) any {
	return fx.Annotate(
		constructor,
		fx.As(new(RouteRegistrar)),
		fx.ResultTags(`group:"routes"`),
	)
}

func NewMux(registrars []RouteRegistrar) *http.ServeMux {
	mux := http.NewServeMux()
	for _, r := range registrars {
		r.RegisterRoutes(mux)
	}
	return mux
}

func NewServer(lc fx.Lifecycle, cfg config.Config, mux *http.ServeMux, log *slog.Logger) *http.Server {
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           Chain(mux, Recover(log), Logging(log), RequestID()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			// Listen síncrono: se a porta estiver ocupada, o start falha
			// na hora, em vez de o erro se perder numa goroutine.
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", srv.Addr, err)
			}
			log.Info("http server started", slog.String("addr", srv.Addr))
			go func() {
				if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Error("http server stopped unexpectedly", slog.Any("error", err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("http server shutting down")
			ctx, cancel := context.WithTimeout(ctx, cfg.ShutdownTimeout)
			defer cancel()
			return srv.Shutdown(ctx)
		},
	})
	return srv
}
