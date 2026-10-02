package domain

import "errors"

// Erros de domínio. As camadas externas usam errors.Is para decidir
// como reagir (ex.: qual status HTTP devolver), sem depender da mensagem.
var (
	ErrInvalidOrder      = errors.New("invalid order")
	ErrInvalidStatus     = errors.New("invalid status")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrOrderNotFound     = errors.New("order not found")
	// ErrConcurrentUpdate indica que outro processo alterou o pedido
	// entre a leitura e a escrita (controle de concorrência otimista).
	ErrConcurrentUpdate = errors.New("order was modified concurrently")
)
