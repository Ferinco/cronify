package api

import "net/http"

// RegisterRoutes adds the full /api/v1/* route table plus /healthz to mux.
// Uses Go 1.22+'s stdlib ServeMux method+wildcard patterns — no external
// router dependency. Split out from NewMux so main.go can register the JSON
// API and the dashboard onto one shared mux/http.Server.
func RegisterRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("POST /api/v1/sync", h.withAuth(h.Sync))
	mux.HandleFunc("GET /api/v1/jobs", h.withAuth(h.ListJobs))
	mux.HandleFunc("GET /api/v1/jobs/{id}", h.withAuth(h.GetJob))
	mux.HandleFunc("GET /api/v1/jobs/{id}/runs", h.withAuth(h.ListRuns))
	mux.HandleFunc("POST /api/v1/jobs/{id}/run", h.withAuth(h.RunNow))
	mux.HandleFunc("POST /api/v1/jobs/{id}/pause", h.withAuth(h.Pause))
	mux.HandleFunc("POST /api/v1/jobs/{id}/resume", h.withAuth(h.Resume))
	mux.HandleFunc("DELETE /api/v1/jobs/{id}", h.withAuth(h.DeleteJob))

	mux.HandleFunc("GET /healthz", h.Health)
}

// NewMux is a convenience wrapper around RegisterRoutes for callers (tests)
// that just want a standalone API mux.
func NewMux(h *Handlers) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)
	return mux
}
