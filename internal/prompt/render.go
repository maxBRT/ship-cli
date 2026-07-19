package prompt

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.md
var templatesFS embed.FS

var templates = template.Must(template.ParseFS(templatesFS, "*.md"))

func render(name string, data any) string {
	var b bytes.Buffer
	if err := templates.ExecuteTemplate(&b, name, data); err != nil {
		panic(fmt.Sprintf("prompt: render %s: %v", name, err))
	}
	return b.String()
}
