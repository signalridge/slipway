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

const maxIncludeDepth = 16

// Render parses a template file as a Go text/template and executes it with the given data.
// Use for .tmpl files that contain {{.Variable}} placeholders.
// Partials from templates/_partials/*.tmpl are automatically available via
// {{ template "partial-name" . }} or the include helper {{ include "name" . }}.
func Render(name string, data any) (string, error) {
	return renderFS(embeddedTemplates, name, data)
}

func renderFS(templateFS fs.FS, name string, data any) (string, error) {
	p := path.Join("templates", name)
	b, err := fs.ReadFile(templateFS, p)
	if err != nil {
		return "", fmt.Errorf("template %q: %w", name, err)
	}

	// The include closure intentionally captures t; the nil guard defends
	// against future reorderings.
	var t *template.Template
	var includeStack []string

	funcMap := template.FuncMap{"include": newIncludeFunc(&t, &includeStack)}

	t, err = template.New(name).Funcs(funcMap).Parse(string(b))
	if err != nil {
		return "", fmt.Errorf("template %q parse: %w", name, err)
	}

	// Load partials from _partials/ directory if they exist.
	partials, _ := fs.Glob(templateFS, "templates/_partials/*.tmpl")
	for _, pp := range partials {
		pb, readErr := fs.ReadFile(templateFS, pp)
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

func newIncludeFunc(tRef **template.Template, includeStack *[]string) func(string, any) (string, error) {
	return func(tmplName string, tmplData any) (string, error) {
		t := *tRef
		if t == nil {
			return "", fmt.Errorf("include %q: template set not initialized", tmplName)
		}
		for _, active := range *includeStack {
			if active == tmplName {
				return "", fmt.Errorf("include %q: cyclic include detected (stack: %v)", tmplName, *includeStack)
			}
		}
		if len(*includeStack) >= maxIncludeDepth {
			return "", fmt.Errorf("include %q: nesting depth %d exceeds maximum %d", tmplName, len(*includeStack), maxIncludeDepth)
		}
		if t.Lookup(tmplName) == nil {
			return "", fmt.Errorf("include %q: template not found", tmplName)
		}
		*includeStack = append(*includeStack, tmplName)
		defer func() { *includeStack = (*includeStack)[:len(*includeStack)-1] }()
		var buf bytes.Buffer
		if err := t.ExecuteTemplate(&buf, tmplName, tmplData); err != nil {
			return "", fmt.Errorf("include %q: %w", tmplName, err)
		}
		return buf.String(), nil
	}
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
