package tmpl

import (
	"embed"
	"fmt"
	"path/filepath"
	"slices"
)

//go:embed templates/*
var embeddedTemplates embed.FS

func Content(name string) (string, error) {
	path := filepath.Join("templates", name)
	b, err := embeddedTemplates.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("template %q: %w", name, err)
	}
	return string(b), nil
}

func Names() []string {
	names := []string{
		"assurance.md",
		"change.yaml",
		"design.md",
		"explore.md",
		"proposal.md",
		"spec.md",
		"tasks.md",
	}
	slices.Sort(names)
	return names
}
