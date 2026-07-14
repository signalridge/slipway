package tmpl

import (
	"embed"
	"fmt"
	"path"
	"strings"
)

//go:embed templates/_partials/*.tmpl
//go:embed templates/skills/*/*.md
//go:embed templates/skills/*/references/*.md
var embeddedTemplates embed.FS

// Content returns the raw content of a template file.
// Use for static templates that need no variable substitution.
func Content(name string) (string, error) {
	p := templatePath(name)
	b, err := embeddedTemplates.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("template %q: %w", name, err)
	}
	return normalizeTemplateLineEndings(string(b)), nil
}

func templatePath(name string) string {
	return path.Join("templates", strings.ReplaceAll(name, "\\", "/"))
}

func normalizeTemplateLineEndings(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	return strings.ReplaceAll(raw, "\r", "\n")
}
