package tmpl

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

//go:embed templates/_partials/*.tmpl
//go:embed templates/skills/*/*.md
//go:embed templates/skills/*/references/*.md
var embeddedTemplates embed.FS

// TemplateFS returns a read-only view of the embedded templates rooted at
// the "templates/" directory. Callers use it to enumerate optional support
// directories like <skill>/references or <skill>/scripts that do not have
// a fixed file list.
func TemplateFS() fs.FS {
	sub, err := fs.Sub(embeddedTemplates, "templates")
	if err != nil {
		// The "templates" sub-FS is embedded at build time; failure here
		// means the embed directive is broken.
		panic(fmt.Errorf("tmpl: fs.Sub: %w", err))
	}
	return sub
}

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
