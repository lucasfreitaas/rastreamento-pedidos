// Package web serve a interface de teste da API. O HTML é embutido no
// binário com go:embed, então não há arquivos extras para publicar.
package web

import (
	_ "embed"
	"net/http"
)

//go:embed index.html
var indexHTML []byte

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// "{$}" faz a rota casar só com "/" exato, e não com qualquer caminho.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(indexHTML)
	})
}
