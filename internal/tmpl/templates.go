package tmpl

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"strings"
	"text/template"
)

//go:embed templates/_partials/*.tmpl
//go:embed templates/skills/*/*.md
//go:embed templates/skills/*/references/*.md
var embeddedTemplates embed.FS

// Content renders a template file with the embedded shared partials.
func Content(name string) (string, error) {
	p := templatePath(name)
	parsed, err := template.New("content").ParseFS(embeddedTemplates, "templates/_partials/*.tmpl")
	if err != nil {
		return "", fmt.Errorf("parse shared template partials: %w", err)
	}
	b, err := embeddedTemplates.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("template %q: %w", name, err)
	}
	if _, err := parsed.New("content").Parse(string(b)); err != nil {
		return "", fmt.Errorf("parse template %q: %w", name, err)
	}
	var rendered bytes.Buffer
	if err := parsed.ExecuteTemplate(&rendered, "content", nil); err != nil {
		return "", fmt.Errorf("render template %q: %w", name, err)
	}
	return normalizeTemplateLineEndings(rendered.String()), nil
}

func templatePath(name string) string {
	return path.Join("templates", strings.ReplaceAll(name, "\\", "/"))
}

func normalizeTemplateLineEndings(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	return strings.ReplaceAll(raw, "\r", "\n")
}
