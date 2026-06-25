package http

import (
	"alloy/models/contract"
	"net/http"
)

// Handlers bundles core's HTTP handlers.
type Handlers struct {
}

func New() *Handlers {
	return &Handlers{}
}
func (h *Handlers) Mount(r contract.Router) {
	r.Route("/auth", func(auth contract.Router) {
		auth.Post("/register", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotImplemented)
		})
	})

}
