package http

import (
	alloy "alloy/internal/platform/alloy"
	"alloy/models/contract"
	"context"
	"net/http"
)

// Handlers bundles core's HTTP handlers.
type Handlers struct {
	app contract.Runtime
	ctx context.Context
}

func New(app contract.Runtime, ctx context.Context) *Handlers {
	return &Handlers{app: app, ctx: ctx}
}
func (h *Handlers) Mount(r contract.Router) {
	r.Get("/helloworld", func(w http.ResponseWriter, r *http.Request) {
		alloy.OK(w, r, "OK")
	})
}
