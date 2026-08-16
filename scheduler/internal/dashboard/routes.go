package dashboard

import "net/http"

// RegisterRoutes adds the dashboard's routes to mux. "/{$}" is Go 1.22's
// exact-root pattern — deliberately not "/", which would match every
// unmatched path and could shadow internal/api's "/api/v1/*" and "/healthz"
// on a shared mux.
func RegisterRoutes(mux *http.ServeMux, h *Handlers) {
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
