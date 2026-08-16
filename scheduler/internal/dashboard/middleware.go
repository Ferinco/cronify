package dashboard

import (
	"crypto/subtle"
	"net/http"
)

// withBasicAuth gates every dashboard route behind CRONIFY_ADMIN_TOKEN,
// reusing the same shared secret internal/api uses via Bearer — CLAUDE.md
// already groups "CLI/dashboard -> scheduler admin API" under one token,
// this just adds Basic as a second transport so a browser can prompt for it
// natively. Username is read but never checked; any username works.
//
// Known tradeoff: browsers cache Basic Auth credentials per-origin and
// auto-attach them to same-origin requests, including ones triggered by
// another site the admin happens to have open (a CSRF-shaped risk). Accepted
// for a single/few-admin self-hosted tool — not a concern for a multi-tenant
// or public-facing deployment, which this project explicitly isn't scoped
// for (see CLAUDE.md's "Explicitly out of scope for v1").
func (h *Handlers) withBasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := r.BasicAuth()
		if !ok || len(pass) != len(h.AdminToken) || subtle.ConstantTimeCompare([]byte(pass), []byte(h.AdminToken)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="cronify"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
