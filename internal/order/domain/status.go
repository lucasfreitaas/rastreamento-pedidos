package domain

// Status representa a etapa atual do pedido.
type Status string

const (
	StatusCreated   Status = "CREATED"
	StatusPaid      Status = "PAID"
	StatusPicking   Status = "PICKING"
	StatusShipped   Status = "SHIPPED"
	StatusDelivered Status = "DELIVERED"
	StatusCanceled  Status = "CANCELED"
)

// transitions define a máquina de estados do pedido.
// Pedidos só podem ser cancelados antes de serem enviados.
var transitions = map[Status][]Status{
	StatusCreated:   {StatusPaid, StatusCanceled},
	StatusPaid:      {StatusPicking, StatusCanceled},
	StatusPicking:   {StatusShipped, StatusCanceled},
	StatusShipped:   {StatusDelivered},
	StatusDelivered: {},
	StatusCanceled:  {},
}

// IsValid indica se o status é conhecido.
func (s Status) IsValid() bool {
	_, ok := transitions[s]
	return ok
}

// CanTransitionTo indica se a transição s -> to é permitida.
func (s Status) CanTransitionTo(to Status) bool {
	for _, allowed := range transitions[s] {
		if allowed == to {
			return true
		}
	}
	return false
}

// NextStatuses retorna os status para os quais o pedido pode ir. A API
// expõe essa lista para que o cliente (ex.: a interface web) não precise
// duplicar a regra de transição.
func (s Status) NextStatuses() []Status {
	return append([]Status(nil), transitions[s]...)
}

// IsFinal indica se o pedido não pode mais mudar de status.
func (s Status) IsFinal() bool {
	return s.IsValid() && len(transitions[s]) == 0
}
