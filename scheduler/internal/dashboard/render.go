package dashboard

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
	"strings"
)

// page is embedded by every template data struct so base.tmpl can render a
// title and an optional flash banner regardless of which page is rendering.
type page struct {
	Title string
	Flash string // already-resolved display text, or "" — never a raw query value
}

func render(w http.ResponseWriter, status int, t *template.Template, data any) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base.tmpl", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w) //nolint:errcheck // response write; nothing meaningful to do with an error here
}

func notFound(w http.ResponseWriter) {
	http.Error(w, "job not found", http.StatusNotFound)
}

func serverError(w http.ResponseWriter, err error) {
	http.Error(w, "internal error: "+err.Error(), http.StatusInternalServerError)
}

// flashMessages is the only set of values the flash banner will ever render —
// readFlash resolves an incoming ?flash= key against this map so the banner
// never echoes arbitrary query-string content back into the page.
var flashMessages = map[string]string{
	"triggered": "Run triggered.",
	"lock_held": "A run is already in progress for this job — skipped.",
	"paused":    "Job paused.",
	"resumed":   "Job resumed.",
	"deleted":   "Job deleted.",
}

func readFlash(r *http.Request) string {
	return flashMessages[r.URL.Query().Get("flash")]
}

// redirectTarget builds a same-origin redirect path for the Post-Redirect-Get
// pattern: it trusts only the Referer header's path+query (never its scheme
// or host), so a forged Referer from a non-browser client can't turn an
// action into an off-site redirect. Preserves the referer's existing query
// (e.g. a job detail page's "?limit=200") and layers the flash param on top.
func redirectTarget(r *http.Request, fallback, flash string) string {
	u := &url.URL{Path: fallback}
	if ref := r.Referer(); ref != "" {
		if parsed, err := url.Parse(ref); err == nil && strings.HasPrefix(parsed.Path, "/") {
			u = &url.URL{Path: parsed.Path, RawQuery: parsed.RawQuery}
		}
	}

	if flash != "" {
		q := u.Query()
		q.Set("flash", flash)
		u.RawQuery = q.Encode()
	}
	return u.String()
}
