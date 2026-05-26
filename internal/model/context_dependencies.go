package model

import (
	"fmt"
	"slices"
	"strings"
)

// ContextRequirement identifies an archived change whose output should be loaded
// as selective prior context for the current change.
type ContextRequirement struct {
	Slug     string   `yaml:"slug,omitempty" json:"slug,omitempty"`
	Provides []string `yaml:"provides,omitempty" json:"provides,omitempty"`
}

func (r *ContextRequirement) Normalize() {
	if r == nil {
		return
	}
	r.Slug = strings.TrimSpace(r.Slug)
	r.Provides = uniqueSortedNonEmpty(r.Provides)
}

func (r ContextRequirement) Validate() error {
	if strings.TrimSpace(r.Slug) == "" {
		return fmt.Errorf("slug is required")
	}
	return nil
}

// ContextDependencies is execution-context metadata that drives prior-context
// selection (context_dependencies.go:28 LoadArchivedChange) and next input
// assembly (next_context_build.go:54). It is NOT consumed by progression,
// governance, or gate logic. Its selective prior context capability must not
// be removed, but it should not be elevated to a gate/governance contract.
type ContextDependencies struct {
	Requires []ContextRequirement `yaml:"requires,omitempty" json:"requires,omitempty"`
}

func (d *ContextDependencies) Normalize() {
	if d == nil {
		return
	}
	for i := range d.Requires {
		d.Requires[i].Normalize()
	}
	slices.SortFunc(d.Requires, func(a, b ContextRequirement) int {
		if a.Slug < b.Slug {
			return -1
		}
		if a.Slug > b.Slug {
			return 1
		}
		return 0
	})
	normalized := make([]ContextRequirement, 0, len(d.Requires))
	seen := map[string]struct{}{}
	for _, req := range d.Requires {
		if strings.TrimSpace(req.Slug) == "" {
			continue
		}
		if _, ok := seen[req.Slug]; ok {
			continue
		}
		seen[req.Slug] = struct{}{}
		normalized = append(normalized, req)
	}
	d.Requires = normalized
}

func (d ContextDependencies) Validate() error {
	for i, req := range d.Requires {
		if err := req.Validate(); err != nil {
			return fmt.Errorf("requires[%d]: %w", i, err)
		}
	}
	return nil
}

func (d ContextDependencies) IsEmpty() bool {
	return len(d.Requires) == 0
}

func uniqueSortedNonEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	slices.Sort(out)
	if len(out) == 0 {
		return nil
	}
	return out
}
