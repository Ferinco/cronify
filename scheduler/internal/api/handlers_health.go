package api

import (
	"net/http"

	"github.com/Ferinco/cronify/scheduler/internal/httpjson"
)

// Health handles GET /healthz. No auth, per convention.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}
