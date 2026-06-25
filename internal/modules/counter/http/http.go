package http

import (
	"alloy/internal/modules/counter/service"
	alloy "alloy/internal/platform/alloy"
	"alloy/models/contract"
	"context"
	"net/http"
)

var (
	count = 0
)

// Handlers bundles core's HTTP handlers.
type Handlers struct {
	app contract.Runtime
	ctx context.Context
	scv service.Service
}

func New(app contract.Runtime, ctx context.Context, svc service.Service) *Handlers {
	return &Handlers{app: app, ctx: ctx, scv: svc}
}
func (h *Handlers) Mount(r contract.Router) {
	r.Get("/add", func(w http.ResponseWriter, r *http.Request) {
		alloy.OK(w, r, h.scv.Add(1))
	})
}
