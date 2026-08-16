package api

import "net/http"

// NewMux builds the full /api/v1/* route table plus /healthz. Uses Go 1.22+'s
// stdlib ServeMux method+wildcard patterns — no external router dependency.
func NewMux(h *Handlers) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/sync", h.withAuth(h.Sync))
	mux.HandleFunc("GET /api/v1/jobs", h.withAuth(h.ListJobs))
	mux.HandleFunc("GET /api/v1/jobs/{id}", h.withAuth(h.GetJob))
	mux.HandleFunc("GET /api/v1/jobs/{id}/runs", h.withAuth(h.ListRuns))
	mux.HandleFunc("POST /api/v1/jobs/{id}/run", h.withAuth(h.RunNow))
	mux.HandleFunc("POST /api/v1/jobs/{id}/pause", h.withAuth(h.Pause))
	mux.HandleFunc("POST /api/v1/jobs/{id}/resume", h.withAuth(h.Resume))
	mux.HandleFunc("DELETE /api/v1/jobs/{id}", h.withAuth(h.DeleteJob))

	mux.HandleFunc("GET /healthz", h.Health)

	return mux
}
