// Package logger configura o log estruturado (JSON) da aplicação.
package logger

import (
	"log/slog"
	"os"
	"strings"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"order-tracking/internal/platform/config"
)

var Module = fx.Module("logger", fx.Provide(New))

// FxLogger faz o próprio Fx registrar seus eventos no slog. Precisa ser
// passado no nível da aplicação (fx.WithLogger dentro de um fx.Module só
// vale para aquele módulo). Eventos rotineiros ficam em DEBUG; erros
// continuam aparecendo.
func FxLogger(l *slog.Logger) fxevent.Logger {
	fl := &fxevent.SlogLogger{Logger: l.With(slog.String("component", "fx"))}
	fl.UseLogLevel(slog.LevelDebug)
	return fl
}

func New(cfg config.Config) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)})
	return slog.New(h).With(
		slog.String("service", "order-api"),
		slog.String("env", cfg.AppEnv),
	)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
