package tmpl

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"
	"text/template"
)

//go:embed templates/_partials/*.tmpl
//go:embed templates/artifacts/*.md
//go:embed templates/commands/*.tmpl
//go:embed templates/hooks/*.tmpl
//go:embed templates/skills/*/*.md templates/skills/*/*.tmpl
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

// ContentIfExists returns a template's raw content when present.
// Missing templates are reported with exists=false and no error.
func ContentIfExists(name string) (content string, exists bool, err error) {
	p := templatePath(name)
	b, err := embeddedTemplates.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("template %q: %w", name, err)
	}
	return normalizeTemplateLineEndings(string(b)), true, nil
}

const maxIncludeDepth = 16

// partialsGlob matches the embedded partial templates.
const partialsGlob = "templates/_partials/*.tmpl"

// partialsRootName owns the parsed templates/_partials/*.tmpl definition set.
// It is never executed and never referenced by any template, so its presence
// in the namespace cannot change a rendered byte; it exists only to carry the
// shared partial definitions that named templates invoke via
// {{ template "partial-name" . }} or {{ include "name" . }}.
const partialsRootName = "__slipway_partials_root__"

// embeddedPartials parses the embedded _partials set exactly once. The result
// is a master template that Render clones (never executing or mutating it)
// before each render, so the re-glob + re-parse of every partial no longer
// happens per call and per-render include state cannot leak between renders.
var embeddedPartials = sync.OnceValues(func() (*template.Template, error) {
	return parsePartialSet(embeddedTemplates)
})

// Render parses a template file as a Go text/template and executes it with the given data.
// Use for .tmpl files that contain {{.Variable}} placeholders.
// Partials from templates/_partials/*.tmpl are automatically available via
// {{ template "partial-name" . }} or the include helper {{ include "name" . }}.
func Render(name string, data any) (string, error) {
	master, err := embeddedPartials()
	if err != nil {
		return "", err
	}
	// Clone the once-parsed partial set so all per-render funcs and state land
	// on the clone; the shared master is never executed or mutated.
	base, err := master.Clone()
	if err != nil {
		return "", fmt.Errorf("template %q clone: %w", name, err)
	}
	return renderNamed(base, embeddedTemplates, name, data)
}

func renderFS(templateFS fs.FS, name string, data any) (string, error) {
	base, err := parsePartialSet(templateFS)
	if err != nil {
		return "", err
	}
	return renderNamed(base, templateFS, name, data)
}

// parsePartialSet builds a template that owns every templates/_partials/*.tmpl
// definition in templateFS. The include helper is registered before Parse so
// partials that call {{ include ... }} parse cleanly; because the partials are
// {{define}}-only, none of them replaces the root body. The result carries
// definitions only: callers add the named template as an associated template
// and execute that, never this root.
func parsePartialSet(templateFS fs.FS) (*template.Template, error) {
	// The include closure intentionally captures t; the nil guard defends
	// against future reorderings.
	var t *template.Template
	var includeStack []string

	funcMap := template.FuncMap{"include": newIncludeFunc(&t, &includeStack)}
	t = template.New(partialsRootName).Funcs(funcMap)

	// Load partials from _partials/ directory if they exist.
	partials, _ := fs.Glob(templateFS, partialsGlob)
	for _, pp := range partials {
		pb, readErr := fs.ReadFile(templateFS, pp)
		if readErr != nil {
			continue
		}
		if _, parseErr := t.Parse(normalizeTemplateLineEndings(string(pb))); parseErr != nil {
			return nil, fmt.Errorf("partial %q parse: %w", pp, parseErr)
		}
	}
	return t, nil
}

// renderNamed parses the named template as an associated template of base
// (which already owns the _partials set), registers a fresh per-render include
// helper on the set, executes the named template, and returns normalized
// output. base must be caller-owned (a clone for the embedded path, or a
// freshly parsed set for an arbitrary FS) and must never be the shared master.
func renderNamed(base *template.Template, templateFS fs.FS, name string, data any) (string, error) {
	p := templatePath(name)
	b, err := fs.ReadFile(templateFS, p)
	if err != nil {
		return "", fmt.Errorf("template %q: %w", name, err)
	}

	// The include closure intentionally captures t; the nil guard defends
	// against future reorderings.
	var t *template.Template
	var includeStack []string

	funcMap := template.FuncMap{"include": newIncludeFunc(&t, &includeStack)}

	t, err = base.New(name).Funcs(funcMap).Parse(normalizeTemplateLineEndings(string(b)))
	if err != nil {
		return "", fmt.Errorf("template %q parse: %w", name, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template %q execute: %w", name, err)
	}
	return normalizeTemplateLineEndings(buf.String()), nil
}

func templatePath(name string) string {
	return path.Join("templates", strings.ReplaceAll(name, "\\", "/"))
}

func normalizeTemplateLineEndings(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	return strings.ReplaceAll(raw, "\r", "\n")
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
