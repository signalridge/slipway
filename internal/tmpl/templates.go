package tmpl

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"text/template"
)

//go:embed all:templates
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
	p := path.Join("templates", name)
	b, err := embeddedTemplates.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("template %q: %w", name, err)
	}
	return string(b), nil
}

// ContentIfExists returns a template's raw content when present.
// Missing templates are reported with exists=false and no error.
func ContentIfExists(name string) (content string, exists bool, err error) {
	p := path.Join("templates", name)
	b, err := embeddedTemplates.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("template %q: %w", name, err)
	}
	return string(b), true, nil
}

// Render parses a template file as a Go text/template and executes it with the given data.
// Use for .tmpl files that contain {{.Variable}} placeholders.
// Partials from templates/_partials/*.tmpl are automatically available via
// {{ template "partial-name" . }}.
func Render(name string, data any) (string, error) {
	p := path.Join("templates", name)
	b, err := embeddedTemplates.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("template %q: %w", name, err)
	}

	t, err := template.New(name).Parse(string(b))
	if err != nil {
		return "", fmt.Errorf("template %q parse: %w", name, err)
	}

	// Load partials from _partials/ directory if they exist.
	partials, _ := fs.Glob(embeddedTemplates, "templates/_partials/*.tmpl")
	for _, pp := range partials {
		pb, readErr := embeddedTemplates.ReadFile(pp)
		if readErr != nil {
			continue
		}
		if _, parseErr := t.Parse(string(pb)); parseErr != nil {
			return "", fmt.Errorf("partial %q parse: %w", pp, parseErr)
		}
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template %q execute: %w", name, err)
	}
	return buf.String(), nil
}

// AgentNames returns the list of agent definition basenames (without extension).
func AgentNames() []string {
	names := []string{
		"slipway-analyst",
		"slipway-planner",
		"slipway-auditor",
		"slipway-orchestrator",
		"slipway-reviewer",
		"slipway-verifier",
		"slipway-closer",
		"slipway-executor",
		"slipway-debugger",
		"slipway-researcher",
		"slipway-mapper",
	}
	slices.Sort(names)
	return names
}
