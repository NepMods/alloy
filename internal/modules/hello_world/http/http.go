package http

import (
	"alloy/internal/modules/hello_world/service"
	alloy "alloy/internal/platform/alloy"
	"alloy/models/contract"
	"context"
	"net/http"
)

// Handlers bundles core's HTTP handlers.
type Handlers struct {
	app contract.Runtime
	ctx context.Context
	svc *service.Service
}

func New(app contract.Runtime, ctx context.Context, svc *service.Service) *Handlers {
	return &Handlers{app: app, ctx: ctx, svc: svc}
}
func (h *Handlers) Mount(r contract.Router) {
	r.Get("/say", func(w http.ResponseWriter, r *http.Request) {
		alloy.OK(w, r, "Hello World")
	})
}
