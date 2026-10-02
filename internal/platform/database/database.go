// Package database cria o pool de conexões com o PostgreSQL e roda as
// migrações, amarrando tudo ao ciclo de vida do Fx.
package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.uber.org/fx"

	"order-tracking/internal/platform/config"
	"order-tracking/migrations"
)

var Module = fx.Module("database",
	fx.Provide(NewPool),
	fx.Invoke(RegisterMigrations),
)

// NewPool cria o pool. A conexão só é testada no OnStart, para que erros
// apareçam na inicialização e não na primeira requisição.
func NewPool(lc fx.Lifecycle, cfg config.Config, log *slog.Logger) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := pool.Ping(ctx); err != nil {
				return fmt.Errorf("ping database: %w", err)
			}
			log.Info("database connected")
			return nil
		},
		OnStop: func(context.Context) error {
			pool.Close()
			log.Info("database pool closed")
			return nil
		},
	})
	return pool, nil
}

// RegisterMigrations roda as migrações pendentes no start. Como o hook é
// registrado depois do hook do pool, o ping já terá acontecido.
func RegisterMigrations(lc fx.Lifecycle, cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) {
	if !cfg.RunMigrations {
		return
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return Migrate(ctx, pool, log)
		},
	})
}

// Migrate aplica as migrações embutidas. Também é usada pelos testes.
func Migrate(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	for _, r := range results {
		log.Info("migration applied",
			slog.String("file", r.Source.Path),
			slog.Int64("duration_ms", r.Duration.Milliseconds()),
		)
	}
	return nil
}
