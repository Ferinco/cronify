package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Ferinco/cronify/scheduler/internal/httpjson"
	"github.com/Ferinco/cronify/scheduler/internal/model"
)

// Sync handles POST /api/v1/sync — full reconciliation for one source, per
// CLAUDE.md: upserts jobs present in the payload, deletes jobs missing from it.
// On a malformed/incomplete body, the error body is returned as-is so it's
// meaningful even though the CLI (sync.ts) surfaces non-2xx bodies as raw text.
func (h *Handlers) Sync(w http.ResponseWriter, r *http.Request) {
	var req model.SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpjson.Error(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.Source == "" {
		httpjson.Error(w, http.StatusBadRequest, `"source" is required`)
		return
	}
	if req.AppURL == "" {
		httpjson.Error(w, http.StatusBadRequest, `"appUrl" is required`)
		return
	}

	result, err := h.Store.ReconcileSource(r.Context(), req.Source, req.AppURL, req.CronSecret, req.Jobs, time.Now(), h.Next)
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}
