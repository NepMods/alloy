package server_models

import (
	"alloy/internal/app/config"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	chi "github.com/go-chi/chi/v5"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// Deps are the runtime services the server needs at construction.
type Deps struct {
	Config config.Config
	Logger func(string)
	DBPing func(ctx context.Context) error
}

type httpRoot struct {
	mux       *chi.Mux
	mounted   map[string]bool
	tenantMWs []func(http.Handler) http.Handler
}

func newHTTPRoot(mux *chi.Mux) *httpRoot {
	return &httpRoot{mux: mux, mounted: map[string]bool{}}
}

// Server is the HTTP entrypoint.
type Server struct {
	cfg    config.Config
	log    func(string)
	router *chi.Mux
	root   *httpRoot // contract.HTTPRoot adapter

	dbPing func(ctx context.Context) error
	srv    *http.Server
}

// New builds the server with the full middleware chain wired. Modules later
// call server.HTTPRoot().Mount(...) to attach their routes.
func New(deps Deps) *Server {
	r := chi.NewRouter()

	// Middleware chain (order matters).
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(recoverer(deps.Logger))
	r.Use(requestLogger(deps.Logger))

	// Main Endpoint
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"We are online baby!"}`))
	})

	// Health endpoints (root-level).
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/ready", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if deps.DBPing != nil {
			if err := deps.DBPing(ctx); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"db_unavailable"}`))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	s := &Server{
		cfg:    deps.Config,
		log:    deps.Logger,
		router: r,
		dbPing: deps.DBPing,
		root:   newHTTPRoot(r),
	}
	return s
}

// Run blocks until SIGTERM, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	s.srv = &http.Server{
		Addr:              addrFor(s.cfg),
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Wait for interrupt.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		s.log("http server starting : " + "addr" + s.srv.Addr + ", env: " + s.cfg.App.Env)
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case sig := <-stop:
		s.log("shutdown signal received : " + sig.String())
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		s.log("graceful shutdown failed : " + err.Error())
		return err
	}
	s.log("http server stopped")
	return nil
}

func addrFor(cfg config.Config) string {
	// net/http expects host:port. ":8080" binds all interfaces.
	return ":" + strconv.Itoa(cfg.App.Port)
}
func requestLogger(log func(string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(ww, r)
			log("-----")
			log("http request")
			log("method: " + r.Method)
			log("path: " + r.URL.Path)
			log("status: " + strconv.Itoa(ww.status))
			log("duration_ms: " + strconv.FormatInt(time.Since(start).Milliseconds(), 10))
			log("ip: " + r.RemoteAddr)
			log("user_agent: " + r.UserAgent())
			log("referer: " + r.Referer())
			log("request_id: " + chimw.GetReqID(r.Context()))
			log("-----")
		})
	}
}

// recoverer catches panics, logs them, and returns a 500.
func recoverer(log func(string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log("panic recovered")
					log("-----")
					log("panic: " + fmt.Sprintf("%v", rec))
					log("stack: " + string(debug.Stack()))
					log("path: " + r.URL.Path)
					log("method: " + r.Method)
					log("-----")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
