package api

import (
	"crypto/subtle"
	"net/http"

	"github.com/Ferinco/cronify/scheduler/internal/httpjson"
)

// withAuth mirrors packages/cronify/src/server.ts's check: length check first,
// then a constant-time comparison — Go's crypto/subtle.ConstantTimeCompare is the
// equivalent of node:crypto.timingSafeEqual, so both ends of the system behave
// symmetrically.
func (h *Handlers) withAuth(next http.HandlerFunc) http.HandlerFunc {
	expected := "Bearer " + h.AdminToken
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("authorization")
		if len(got) != len(expected) || subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			httpjson.Error(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		next(w, r)
	}
}
