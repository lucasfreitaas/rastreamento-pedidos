// Package config carrega as configurações a partir de variáveis de ambiente.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"go.uber.org/fx"
)

var Module = fx.Module("config", fx.Provide(Load))

type Config struct {
	AppEnv          string
	HTTPAddr        string
	DatabaseURL     string
	RunMigrations   bool
	LogLevel        string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:          getEnv("APP_ENV", "local"),
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		RunMigrations:   true,
		ShutdownTimeout: 10 * time.Second,
	}

	if v := os.Getenv("RUN_MIGRATIONS"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("RUN_MIGRATIONS: %w", err)
		}
		cfg.RunMigrations = b
	}
	if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err)
		}
		cfg.ShutdownTimeout = d
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.HTTPAddr == "" {
		errs = append(errs, errors.New("HTTP_ADDR cannot be empty"))
	}
	return errors.Join(errs...)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
