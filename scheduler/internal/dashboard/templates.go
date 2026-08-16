package dashboard

import (
	"embed"
	"html/template"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// Each page is parsed as its own template set (base.tmpl + exactly one page
// file). Parsing every *.tmpl into a single shared set would collide: every
// page defines a block named "content", and within one template.Template,
// same-named {{define}} blocks silently overwrite each other — the last one
// parsed would win for every page, regardless of which page is rendering.
func parsePage(file string) *template.Template {
	return template.Must(template.New("base.tmpl").Funcs(funcMap).ParseFS(templatesFS, "templates/base.tmpl", "templates/"+file))
}

var (
	homeTmpl          = parsePage("home.tmpl")
	jobDetailTmpl     = parsePage("job_detail.tmpl")
	deleteConfirmTmpl = parsePage("delete_confirm.tmpl")
)
