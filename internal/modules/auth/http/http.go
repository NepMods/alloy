package http

import (
	"context"
	"net/http"

	"alloy/internal/modules/auth/service"
	alloy "alloy/internal/platform/alloy"
	"alloy/models/contract"
)

type Handlers struct {
	app contract.Runtime
	ctx context.Context
	svc *service.Service
}

func New(app contract.Runtime, ctx context.Context, svc *service.Service) *Handlers {
	return &Handlers{app: app, ctx: ctx, svc: svc}
}

func (h *Handlers) Mount(r contract.Router) {
	r.Post("/register", h.register)
	r.Post("/login", h.login)
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (h *Handlers) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := alloy.DecodeJSON(r, &req); err != nil {
		alloy.Problem(w, r, err)
		return
	}
	result, err := h.svc.Register(req.Email, req.Password, req.Name)
	if err != nil {
		alloy.Problem(w, r, alloy.BadRequest("registration_failed", err.Error()))
		return
	}
	result.User.PasswordHash = ""
	alloy.OK(w, r, result)
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := alloy.DecodeJSON(r, &req); err != nil {
		alloy.Problem(w, r, err)
		return
	}
	result, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		alloy.Problem(w, r, alloy.Unauthorized(err.Error()))
		return
	}
	result.User.PasswordHash = ""
	alloy.OK(w, r, result)
}
