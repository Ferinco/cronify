package dashboard

import (
	"crypto/subtle"
	"net"
	"net/http"
	"sync"
	"time"
)

// withBasicAuth gates every dashboard route behind CRONIFY_ADMIN_TOKEN,
// reusing the same shared secret internal/api uses via Bearer — CLAUDE.md
// already groups "CLI/dashboard -> scheduler admin API" under one token,
// this just adds Basic as a second transport so a browser can prompt for it
// natively. Username is read but never checked; any username works — a
// username field only strengthens auth if it's itself a secret, and if it
// were, it'd just be a second weaker copy of the same idea as one strong
// token. The real gate is the password matching AdminToken.
//
// Known tradeoff: browsers cache Basic Auth credentials per-origin and
// auto-attach them to same-origin requests, including ones triggered by
// another site the admin happens to have open (a CSRF-shaped risk). Accepted
// for a single/few-admin self-hosted tool — not a concern for a multi-tenant
// or public-facing deployment, which this project explicitly isn't scoped
// for (see CLAUDE.md's "Explicitly out of scope for v1").
//
// Failed attempts are throttled per source address with growing delays (see
// loginThrottle below) — nothing here enforces AdminToken's strength itself
// (config.WeakAdminToken only warns at startup), so this is the backstop
// against a weak or guessed token being brute-forced quickly.
func (h *Handlers) withBasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := clientKey(r)
		_, pass, ok := r.BasicAuth()
		if !ok || len(pass) != len(h.AdminToken) || subtle.ConstantTimeCompare([]byte(pass), []byte(h.AdminToken)) != 1 {
			time.Sleep(h.loginThrottle.penalize(key))
			w.Header().Set("WWW-Authenticate", `Basic realm="cronify"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		h.loginThrottle.clear(key)
		next(w, r)
	}
}

// clientKey groups repeated attempts by source IP, stripping the ephemeral
// port. Reads r.RemoteAddr only — not X-Forwarded-For, which a client can
// set to anything unless a specific trusted reverse proxy strips/overwrites
// it first, and this project supports several deploy targets (Render,
// Railway, Fly, plain docker-compose) with no shared, verified proxy config
// to trust. Tradeoff: behind a platform whose edge always presents the same
// RemoteAddr to the app, every client collapses into one throttle bucket —
// still strictly better than no throttling, but worth knowing about.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// loginThrottle tracks failed Basic Auth attempts per clientKey and returns
// a growing delay to make brute-forcing a weak/guessed AdminToken slower,
// without ever locking a source out outright — an outright lockout would
// let an attacker deny the real admin access just by failing login from
// wherever the admin's own traffic appears to originate (see clientKey's
// collapsed-RemoteAddr tradeoff above), which would be worse than the
// problem being solved.
type loginThrottle struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
}

type loginAttempt struct {
	count int
	last  time.Time
}

const (
	throttleBase       = 250 * time.Millisecond
	throttleMax        = 4 * time.Second
	throttleResetAfter = 5 * time.Minute
)

// penalize records a failed attempt for key and returns how long to sleep
// before responding. Delay doubles per consecutive recent failure (250ms,
// 500ms, 1s, 2s, 4s, capped), and resets if key has been quiet for
// throttleResetAfter — a stale bucket shouldn't keep punishing a reused or
// dynamically-assigned IP indefinitely.
func (t *loginThrottle) penalize(key string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.attempts == nil {
		t.attempts = make(map[string]*loginAttempt)
	}

	now := time.Now()
	a, ok := t.attempts[key]
	if !ok || now.Sub(a.last) > throttleResetAfter {
		a = &loginAttempt{}
		t.attempts[key] = a
	}
	a.count++
	a.last = now

	shift := a.count - 1
	if shift > 4 { // throttleBase << 4 == throttleMax
		shift = 4
	}
	delay := throttleBase << shift
	if delay > throttleMax {
		delay = throttleMax
	}
	return delay
}

// clear drops key's failure history after a successful auth, so the next
// mistake from that source starts the delay back at throttleBase instead of
// staying penalized for past failures.
func (t *loginThrottle) clear(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.attempts, key)
}
