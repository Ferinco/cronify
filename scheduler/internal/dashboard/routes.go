package dashboard

import "net/http"

// RegisterRoutes adds the dashboard's routes to mux. "/{$}" is Go 1.22's
// exact-root pattern — deliberately not "/", which would match every
// unmatched path and could shadow internal/api's "/api/v1/*" and "/healthz"
// on a shared mux.
//
// h.loginThrottle is allocated here rather than lazily on first request:
// RegisterRoutes always runs to completion, single-threaded, before the
// server starts accepting connections, so this write happens-before every
// request-handling goroutine that later reads it — allocating it lazily
// inside a handler would instead race if two requests hit an unauthenticated
// route at the same moment on a freshly-started server.
func RegisterRoutes(mux *http.ServeMux, h *Handlers) {
	h.loginThrottle = &loginThrottle{}

	mux.HandleFunc("GET /{$}", h.withBasicAuth(h.Home))
	mux.HandleFunc("GET /jobs/{id}", h.withBasicAuth(h.JobDetail))
	mux.HandleFunc("POST /jobs/{id}/run", h.withBasicAuth(h.RunNow))
	mux.HandleFunc("POST /jobs/{id}/pause", h.withBasicAuth(h.Pause))
	mux.HandleFunc("POST /jobs/{id}/resume", h.withBasicAuth(h.Resume))
	mux.HandleFunc("GET /jobs/{id}/delete", h.withBasicAuth(h.DeleteConfirm))
	mux.HandleFunc("POST /jobs/{id}/delete", h.withBasicAuth(h.Delete))
}

// NewMux is a convenience wrapper around RegisterRoutes for callers (tests)
// that just want a standalone dashboard mux.
func NewMux(h *Handlers) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)
	return mux
}
