package dashboard

import (
	"fmt"
	"html/template"
	"math"
	"time"
)

var funcMap = template.FuncMap{
	"relativeTime": relativeTime,
	"rfc3339":      func(t time.Time) string { return t.UTC().Format(time.RFC3339) },
}

// relativeTime renders a coarse, human-friendly offset from now: "in 3h",
// "12m ago", "just now". Stdlib time only — see CLAUDE.md's "two direct Go
// dependencies total" note for why go-humanize isn't used here even though
// it's present as an indirect transitive dependency.
func relativeTime(t time.Time) string {
	d := time.Until(t)
	future := d >= 0
	if !future {
		d = -d
	}

	var text string
	switch {
	case d < 45*time.Second:
		return "just now"
	case d < time.Hour:
		m := int(math.Round(d.Minutes()))
		text = fmt.Sprintf("%dm", m)
	case d < 24*time.Hour:
		h := int(math.Round(d.Hours()))
		text = fmt.Sprintf("%dh", h)
	default:
		days := int(math.Round(d.Hours() / 24))
		text = fmt.Sprintf("%dd", days)
	}

	if future {
		return "in " + text
	}
	return text + " ago"
}
