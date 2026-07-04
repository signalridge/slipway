// Package capability owns two runtime authorities:
//
//  1. The catalog-skill registry (internal identity): bindings for hosts,
//     command-auto routes, technique hints, and export-only metadata.
//     Catalog-rendered SKILL.md frontmatter mirrors this registry and the
//     binding-compare gate enforces 1:1 equality. Workflow-owned hosts may
//     share a catalog ID for command-auto metadata while toolgen owns their
//     exported host template.
//  2. The surface policy registry (public exposure), owned by surfaces.go.
//     It classifies each user-facing selector as primary / explicit focus
//     and resolves public aliases to their backing skill id. --focus
//     resolution reads surfaces.go, not catalog bindings, so BindingCommandAuto
//     metadata is never equivalent to a public selector.
//
// Catalog skills do not replace the governance kernel. ResolveNextSkill in
// internal/engine/progression remains the only progression authority; this
// package emits support attachments, routed-command selections, and bounded
// suggested capabilities that the kernel's host is free to consume or ignore.
package capability

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Tier names the semantic role of a catalog skill.
type Tier string

const (
	TierT1 Tier = "T1" // reusable core method
	TierT2 Tier = "T2" // specialist route (tool-recipe)
	TierT3 Tier = "T3" // diagnostic view
)

// Domain names one of nine concern areas. Schema-lint rejects other values.
type Domain string

const (
	DomainIntake            Domain = "intake"
	DomainExecution         Domain = "execution"
	DomainDebugging         Domain = "debugging"
	DomainReviewQuality     Domain = "review-quality"
	DomainReviewSecurity    Domain = "review-security"
	DomainReviewChangeShape Domain = "review-change-shape"
	DomainVerification      Domain = "verification"
	DomainRepairCI          Domain = "repair-ci"
	DomainOpsDiagnostics    Domain = "ops-diagnostics"
)

// AttachmentMode names a frozen injection shape.
type AttachmentMode string

const (
	AttachmentPosture      AttachmentMode = "posture"
	AttachmentProcedure    AttachmentMode = "procedure"
	AttachmentChecklist    AttachmentMode = "checklist"
	AttachmentToolRecipe   AttachmentMode = "tool-recipe"
	AttachmentReportSchema AttachmentMode = "report-schema"
)

// BindingType names the runtime attachment surface for a binding.
type BindingType string

const (
	BindingHostEmbedded  BindingType = "host-embedded"
	BindingCommandAuto   BindingType = "command-auto"
	BindingTechniqueHint BindingType = "technique-hint"
	BindingExportOnly    BindingType = "export-only"
)

// EvidenceContract names the evidence shape the skill produces.
type EvidenceContract string

const (
	EvidenceVerdict   EvidenceContract = "verdict"
	EvidenceArtifact  EvidenceContract = "artifact"
	EvidenceChecklist EvidenceContract = "checklist"
)

// Binding is a single runtime attachment for a catalog skill.
type Binding struct {
	Type       BindingType    `yaml:"type"`
	Target     string         `yaml:"target"`
	Attachment AttachmentMode `yaml:"attachment"`
}

// HostCapabilityContract declares a host/runtime capability needed to run a
// selected skill with its intended evidence semantics.
type HostCapabilityContract struct {
	Capability          string   `yaml:"capability"`
	Required            bool     `yaml:"required"`
	FallbackModes       []string `yaml:"fallback_modes,omitempty"`
	EvidenceRequirement string   `yaml:"evidence_requirement"`
	Remediation         string   `yaml:"remediation"`
}

// Skill is the authoritative runtime record for one catalog skill.
//
// The fields mirror the source skill frontmatter contract. The binding-compare
// gate uses this record as the source of truth against generated host
// frontmatter.
type Skill struct {
	ID                string
	Domain            Domain
	Function          string
	Tier              Tier
	PrimaryAttachment AttachmentMode
	Summary           string
	Evidence          EvidenceContract
	Bindings          []Binding
	HostCapabilities  []HostCapabilityContract
	HydrateReferences []HydrateReference
}

// HydrateReference is a typed, registry-owned record that mirrors a skill's
// authoring-side `hydrate_references:` frontmatter entry. `Name` is a bare
// basename under the skill's `references/` directory; runtime outputs use the
// collision-safe skill-relative key `<skill-id>/<name>`.
type HydrateReference struct {
	Name   string
	Reason string
}

// Registry exposes read-only lookups over the registered catalog skills.
type Registry struct {
	byID map[string]Skill
}

// NewRegistry builds a Registry over the supplied skills. Duplicate IDs
// return an error; empty registries are legal (useful during bootstrap).
func NewRegistry(skills ...Skill) (*Registry, error) {
	reg := &Registry{byID: make(map[string]Skill, len(skills))}
	for _, sk := range skills {
		id := strings.TrimSpace(sk.ID)
		if id == "" {
			return nil, fmt.Errorf("capability: skill has empty id")
		}
		if _, dup := reg.byID[id]; dup {
			return nil, fmt.Errorf("capability: duplicate skill id %q", id)
		}
		if err := validateSkill(sk); err != nil {
			return nil, fmt.Errorf("capability: skill %q: %w", id, err)
		}
		reg.byID[id] = sk
	}
	return reg, nil
}

// defaultRegistry memoizes the shipped catalog registry. Sharing one instance
// is safe: byID is never written after NewRegistry, no method mutates it, and
// Lookup/All hand back cloned skills so a caller cannot mutate the shared state.
var defaultRegistry = sync.OnceValue(func() *Registry {
	reg, err := NewRegistry(defaultSkills()...)
	if err != nil {
		// defaultSkills is table data, any error is a programmer bug.
		panic(err)
	}
	return reg
})

// DefaultRegistry returns the shipped catalog registry in deterministic order.
func DefaultRegistry() *Registry {
	return defaultRegistry()
}

// clone returns a copy of the skill whose mutable slice fields — Bindings,
// HydrateReferences, HostCapabilities, and each HostCapabilityContract's nested
// FallbackModes — are independently allocated, so a caller mutating the result
// cannot reach back into the memoized registry's stored skills. Every other
// field is a scalar or string (EvidenceContract is a string) and needs no copy.
func (s Skill) clone() Skill {
	s.Bindings = slices.Clone(s.Bindings)
	s.HydrateReferences = slices.Clone(s.HydrateReferences)
	if s.HostCapabilities != nil {
		caps := slices.Clone(s.HostCapabilities)
		for i := range caps {
			caps[i].FallbackModes = slices.Clone(caps[i].FallbackModes)
		}
		s.HostCapabilities = caps
	}
	return s
}

// Lookup returns the skill with the given id. The second return value is false
// when the id is not registered. The returned skill is an independent copy;
// mutating its slice fields does not affect the shared registry.
func (r *Registry) Lookup(id string) (Skill, bool) {
	if r == nil {
		return Skill{}, false
	}
	sk, ok := r.byID[id]
	if !ok {
		return Skill{}, false
	}
	return sk.clone(), true
}

// All returns every registered skill sorted by id. The returned slice and each
// skill's mutable slice fields are freshly allocated, so the caller may mutate
// the result without affecting the shared registry.
func (r *Registry) All() []Skill {
	if r == nil {
		return nil
	}
	out := make([]Skill, 0, len(r.byID))
	for _, sk := range r.byID {
		out = append(out, sk.clone())
	}
	slices.SortFunc(out, func(a, b Skill) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	return out
}

// IDs returns the sorted list of registered skill ids.
func (r *Registry) IDs() []string {
	if r == nil {
		return nil
	}
	ids := make([]string, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// Len returns the number of registered catalog skills.
func (r *Registry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.byID)
}

func validateSkill(sk Skill) error {
	if !validDomain(sk.Domain) {
		return fmt.Errorf("invalid domain %q", sk.Domain)
	}
	if !validTier(sk.Tier) {
		return fmt.Errorf("invalid tier %q", sk.Tier)
	}
	if !validAttachment(sk.PrimaryAttachment) {
		return fmt.Errorf("invalid primary_attachment %q", sk.PrimaryAttachment)
	}
	if !validEvidence(sk.Evidence) {
		return fmt.Errorf("invalid evidence_contract %q", sk.Evidence)
	}
	if strings.TrimSpace(sk.Function) == "" {
		return fmt.Errorf("empty function")
	}
	if strings.TrimSpace(sk.Summary) == "" {
		return fmt.Errorf("empty summary")
	}
	if len(sk.Bindings) == 0 {
		if _, ok := ExplicitFocusBackingIDs()[sk.ID]; !ok {
			return fmt.Errorf("no bindings declared")
		}
	}
	for i, b := range sk.Bindings {
		if !validBindingType(b.Type) {
			return fmt.Errorf("binding[%d]: invalid type %q", i, b.Type)
		}
		if strings.TrimSpace(b.Target) == "" {
			return fmt.Errorf("binding[%d]: empty target", i)
		}
		if !validAttachment(b.Attachment) {
			return fmt.Errorf("binding[%d]: invalid attachment %q", i, b.Attachment)
		}
	}
	for i, cap := range sk.HostCapabilities {
		if strings.TrimSpace(cap.Capability) == "" {
			return fmt.Errorf("host_capabilities[%d]: empty capability", i)
		}
		if strings.TrimSpace(cap.EvidenceRequirement) == "" {
			return fmt.Errorf("host_capabilities[%d]: empty evidence_requirement", i)
		}
		if strings.TrimSpace(cap.Remediation) == "" {
			return fmt.Errorf("host_capabilities[%d]: empty remediation", i)
		}
		for j, fallback := range cap.FallbackModes {
			if strings.TrimSpace(fallback) == "" {
				return fmt.Errorf("host_capabilities[%d].fallback_modes[%d]: empty fallback mode", i, j)
			}
		}
	}
	return nil
}

func validDomain(d Domain) bool {
	switch d {
	case DomainIntake, DomainExecution, DomainDebugging,
		DomainReviewQuality, DomainReviewSecurity, DomainReviewChangeShape,
		DomainVerification, DomainRepairCI, DomainOpsDiagnostics:
		return true
	}
	return false
}

func validTier(t Tier) bool {
	switch t {
	case TierT1, TierT2, TierT3:
		return true
	}
	return false
}

func validAttachment(m AttachmentMode) bool {
	switch m {
	case AttachmentPosture, AttachmentProcedure, AttachmentChecklist,
		AttachmentToolRecipe, AttachmentReportSchema:
		return true
	}
	return false
}

func validEvidence(e EvidenceContract) bool {
	switch e {
	case EvidenceVerdict, EvidenceArtifact, EvidenceChecklist:
		return true
	}
	return false
}

func validBindingType(t BindingType) bool {
	switch t {
	case BindingHostEmbedded, BindingCommandAuto,
		BindingTechniqueHint, BindingExportOnly:
		return true
	}
	return false
}
