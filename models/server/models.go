package server_models

import (
	"alloy/internal/app/contract"
	"context"
	"net/http"

	chi "github.com/go-chi/chi/v5"
	goredis "github.com/redis/go-redis/v9"
)

type httpRoot struct {
	mux       *chi.Mux
	mounted   map[string]bool
	tenantMWs []func(http.Handler) http.Handler
}

// Server is the HTTP entrypoint.
type Server struct {
	log    contract.Logger
	rdb    goredis.UniversalClient
	router *chi.Mux
	root   *httpRoot // contract.HTTPRoot adapter

	dbPing func(ctx context.Context) error
	srv    *http.Server
}
