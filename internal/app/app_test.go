package app_test

import (
	"testing"

	"go.uber.org/fx"

	"order-tracking/internal/app"
)

// TestDependencyGraph garante que todas as dependências do Fx estão
// registradas, sem abrir conexões nem subir o servidor.
func TestDependencyGraph(t *testing.T) {
	if err := fx.ValidateApp(app.Options()); err != nil {
		t.Fatalf("invalid fx graph: %v", err)
	}
}
