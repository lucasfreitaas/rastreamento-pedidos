package main

import (
	"go.uber.org/fx"

	"order-tracking/internal/app"
)

func main() {
	// Run inicia todos os hooks OnStart, espera SIGINT/SIGTERM e então
	// executa os OnStop na ordem inversa (graceful shutdown).
	fx.New(app.Options()).Run()
}
