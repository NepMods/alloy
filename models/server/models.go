package server_models

import (
	"alloy/internal/app/config"
	"alloy/models/contract"
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
	goredis "github.com/redis/go-redis/v9"
)

// Deps are the runtime services the server needs at construction.
type Deps struct {
	Config config.Config
	Logger func(string)

	Redis  goredis.UniversalClient
	DBPing func(ctx context.Context) error
}

type httpRoot struct {
	mux     *chi.Mux
	mounted map[string]bool
}

func (h *httpRoot) Mount(module string, build func(router contract.Router), log func(string)) {
	if h.mounted[module] {
		return // defensive: a module mounting twice is a no-op (Validate should catch earlier)
	}
	h.mounted[module] = true
	h.mux.Route("/v1/"+module, func(r chi.Router) {
		build(&routerAdapter{r: r})
	})
}

// routerAdapter adapts chi.Router to contract.Router.
type routerAdapter struct{ r chi.Router }

func (a *routerAdapter) Get(p string, h any)    { a.r.Get(p, toHandler(h)) }
func (a *routerAdapter) Post(p string, h any)   { a.r.Post(p, toHandler(h)) }
func (a *routerAdapter) Put(p string, h any)    { a.r.Put(p, toHandler(h)) }
func (a *routerAdapter) Patch(p string, h any)  { a.r.Patch(p, toHandler(h)) }
func (a *routerAdapter) Delete(p string, h any) { a.r.Delete(p, toHandler(h)) }

func (a *routerAdapter) Route(pattern string, fn func(r contract.Router)) contract.Router {
	sub := a.r.Route(pattern, func(r chi.Router) { fn(&routerAdapter{r: r}) })
	return &routerAdapter{r: sub}
}

func (a *routerAdapter) Group(fn func(r contract.Router)) contract.Router {
	g := a.r.Group(func(r chi.Router) { fn(&routerAdapter{r: r}) })
	return &routerAdapter{r: g}
}

func (a *routerAdapter) Use(mw ...any) {
	for _, m := range mw {
		if cm, ok := m.(func(http.Handler) http.Handler); ok {
			a.r.Use(cm)
		}
	}
}

// toHandler accepts either an http.HandlerFunc or http.Handler.
func toHandler(h any) http.HandlerFunc {
	switch v := h.(type) {
	case http.HandlerFunc:
		return v
	case func(http.ResponseWriter, *http.Request):
		return v
	case http.Handler:
		return v.ServeHTTP
	default:
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":{"code":"internal","message":"bad handler"}}`, http.StatusInternalServerError)
		}
	}
}

func (h *httpRoot) Router() contract.Router { return &routerAdapter{r: h.mux} }

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

	var startupErr error
	errCh := make(chan error, 1)
	go func() {
		s.log("http server starting : " + "addr" + s.srv.Addr + ", env: " + s.cfg.App.Env)

		// 👇 Insert here to log all routes right before the engine blocks on listening
		logRoutes(s.router, s.log)

		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		s.log("context cancelled, shutting down : " + ctx.Err().Error())
	case sig := <-stop:
		s.log("shutdown signal received : " + sig.String())
	case err := <-errCh:
		s.log("server error : " + err.Error())
		startupErr = err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		s.log("graceful shutdown failed : " + err.Error())
		if startupErr != nil {
			return startupErr
		}
		return err
	}
	s.log("http server stopped")
	if startupErr != nil {
		return startupErr
	}
	return nil
}
func logRoutes(router chi.Router, log func(string)) {
	// Notice the changed type for the middlewares variadic argument
	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		// Formats into clean lines like: "ROUTE: [GET] /ready"
		log(fmt.Sprintf("ROUTE: [%s] %s", method, route))
		return nil
	}

	if err := chi.Walk(router, walkFunc); err != nil {
		log("failed to walk chi router: " + err.Error())
	}
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

// HTTPRoot exposes the mount point modules use to register routes.
func (s *Server) HTTPRoot() contract.HTTPRoot { return s.root }
