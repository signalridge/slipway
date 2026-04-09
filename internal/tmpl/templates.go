package tmpl

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"text/template"
)

//go:embed all:templates
var embeddedTemplates embed.FS

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
