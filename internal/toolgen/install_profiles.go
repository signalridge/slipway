package toolgen

import (
	"fmt"
	"slices"
	"strings"
)

type SkillInstallProfile string

const (
	SkillInstallProfileCore SkillInstallProfile = "core"
	SkillInstallProfileFull SkillInstallProfile = "full"
)

type generatedSkillKind string

const (
	generatedSkillKindHost    generatedSkillKind = "host"
	generatedSkillKindCommand generatedSkillKind = "command"
	generatedSkillKindRouter  generatedSkillKind = "router"
)

type skillInstallMetadata struct {
	PublicName    string
	SourceID      string
	Kind          generatedSkillKind
	Profiles      []SkillInstallProfile
	Requires      []string
	AlwaysInstall bool
}

type skillInstallClosure struct {
	Skills     []skillInstallMetadata
	hostIDs    map[string]struct{}
	commandIDs map[string]struct{}
	routerIDs  map[string]struct{}
}

type namespaceRouterDefinition struct {
	ID           string
	Title        string
	Summary      string
	CommandIDs   []string
	HostSkillIDs []string
	Notes        []string
}

var namespaceRouterDefinitions = []namespaceRouterDefinition{
	{
		ID:      "surface-recovery",
		Title:   "Recovery Surfaces",
		Summary: "Use when a governed change needs recovery, cancellation, checkpoint, or repair routing without bypassing the lifecycle.",
		CommandIDs: []string{
			"repair",
			"fix",
			"checkpoint",
			"cancel",
			"delete",
			"abort",
			"preset",
		},
		HostSkillIDs: []string{
			"root-cause-tracing",
			"git-recovery",
			"worktree-preflight",
		},
		Notes: []string{
			"Use `slipway next --json` or the named command surface before loading a host skill.",
			"`delete`, `cancel`, `abort`, and `preset` remain lifecycle commands; this router only helps choose the command.",
		},
	},
	{
		ID:      "surface-diagnostics",
		Title:   "Diagnostic Surfaces",
		Summary: "Use when you need status, health, statistics, learning, or generated-authoring diagnostics without changing lifecycle state.",
		CommandIDs: []string{
			"status",
			"health",
			"validate",
			"learn",
			"stats",
			"instructions",
		},
		HostSkillIDs: []string{
			"context-assembly",
		},
		Notes: []string{
			"Incident handling is reached through `slipway status --focus incident` or `slipway health --focus incident`.",
			"Diagnostic commands report state; they do not close evidence gaps by themselves.",
		},
	},
	{
		ID:      "surface-review-quality",
		Title:   "Review And Validation Surfaces",
		Summary: "Use when review, validation, SAST, coverage, property, or mutation-testing support is needed after the governed command selects the route.",
		CommandIDs: []string{
			"review",
			"validate",
			"evidence",
		},
		HostSkillIDs: []string{
			"spec-compliance-review",
			"code-quality-review",
			"independent-review",
			"security-review",
			"goal-verification",
			"spec-trace",
			"coverage-analysis",
			"test-design",
		},
		Notes: []string{
			"Security review stays directly installed and selected by policy; do not use this router as a substitute for that gate.",
			"Optional SAST, calibration, property, and mutation flows are selected by `--focus` surfaces.",
		},
	},
}

var alwaysInstalledCommandIDs = []string{
	"new",
	"intake",
	"plan",
	"implement",
	"review",
	"fix",
	"done",
	"next",
	"run",
	"status",
	"init",
	"codebase-map",
	"validate",
	"repair",
	"evidence",
}

var installRequiresByPublicName = map[string][]string{
	adapterSkillName(workflowSkillID): commandPublicNames(alwaysInstalledCommandIDs...),

	adapterSkillName("codebase-map"): {adapterSkillName("codebase-mapping")},
	adapterSkillName("fix"):          {adapterSkillName("root-cause-tracing")},
	adapterSkillName("implement"):    {adapterSkillName("wave-orchestration")},
	adapterSkillName("intake"):       {adapterSkillName("intake-clarification")},
	adapterSkillName("plan"):         {adapterSkillName("research-orchestration"), adapterSkillName("plan-audit")},
	adapterSkillName("review"): {
		adapterSkillName("spec-compliance-review"),
		adapterSkillName("code-quality-review"),
		adapterSkillName("independent-review"),
		adapterSkillName("security-review"),
		adapterSkillName("goal-verification"),
	},
	adapterSkillName("run"):      {adapterSkillName("next")},
	adapterSkillName("validate"): {adapterSkillName("spec-trace"), adapterSkillName("coverage-analysis")},

	adapterSkillName("code-quality-review"):    {adapterSkillName("coding-discipline"), adapterSkillName("test-design")},
	adapterSkillName("final-closeout"):         {adapterSkillName("done"), adapterSkillName("status"), adapterSkillName("validate")},
	adapterSkillName("goal-verification"):      {adapterSkillName("validate"), adapterSkillName("coverage-analysis")},
	adapterSkillName("plan-audit"):             {adapterSkillName("context-assembly"), adapterSkillName("coding-discipline"), adapterSkillName("validate")},
	adapterSkillName("research-orchestration"): {adapterSkillName("codebase-map"), adapterSkillName("codebase-mapping"), adapterSkillName("context-assembly")},
	adapterSkillName("security-review"):        {adapterSkillName("review"), adapterSkillName("validate")},
	adapterSkillName("spec-compliance-review"): {adapterSkillName("coding-discipline"), adapterSkillName("spec-trace")},
	adapterSkillName("wave-orchestration"): {
		adapterSkillName("coding-discipline"),
		adapterSkillName("root-cause-tracing"),
		adapterSkillName("test-design"),
	},
	adapterSkillName("worktree-preflight"): {adapterSkillName("git-recovery")},
}

func commandPublicNames(ids ...string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, adapterSkillName(id))
	}
	return out
}

func installProfileClosure(profile SkillInstallProfile) (skillInstallClosure, error) {
	normalized, err := normalizeSkillInstallProfile(profile)
	if err != nil {
		return skillInstallClosure{}, err
	}

	all := skillInstallMetadataByPublicName()
	selected := map[string]struct{}{}
	queue := []string{}
	for _, meta := range all {
		if meta.AlwaysInstall || slices.Contains(meta.Profiles, normalized) {
			if _, exists := selected[meta.PublicName]; exists {
				continue
			}
			selected[meta.PublicName] = struct{}{}
			queue = append(queue, meta.PublicName)
		}
	}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		meta, ok := all[name]
		if !ok {
			return skillInstallClosure{}, fmt.Errorf("install profile %q selected unknown skill %q", normalized, name)
		}
		for _, required := range meta.Requires {
			if _, ok := all[required]; !ok {
				return skillInstallClosure{}, fmt.Errorf("install profile %q skill %q requires unknown skill %q", normalized, name, required)
			}
			if _, exists := selected[required]; exists {
				continue
			}
			selected[required] = struct{}{}
			queue = append(queue, required)
		}
	}

	closure := skillInstallClosure{
		hostIDs:    map[string]struct{}{},
		commandIDs: map[string]struct{}{},
		routerIDs:  map[string]struct{}{},
	}
	for name := range selected {
		closure.Skills = append(closure.Skills, all[name])
	}
	slices.SortFunc(closure.Skills, func(a, b skillInstallMetadata) int {
		if a.PublicName < b.PublicName {
			return -1
		}
		if a.PublicName > b.PublicName {
			return 1
		}
		return 0
	})
	for _, meta := range closure.Skills {
		switch meta.Kind {
		case generatedSkillKindHost:
			closure.hostIDs[meta.SourceID] = struct{}{}
		case generatedSkillKindCommand:
			closure.commandIDs[meta.SourceID] = struct{}{}
		case generatedSkillKindRouter:
			closure.routerIDs[meta.SourceID] = struct{}{}
		}
	}
	return closure, nil
}

func normalizeSkillInstallProfile(profile SkillInstallProfile) (SkillInstallProfile, error) {
	switch strings.TrimSpace(string(profile)) {
	case "", string(SkillInstallProfileFull):
		return SkillInstallProfileFull, nil
	case string(SkillInstallProfileCore):
		return SkillInstallProfileCore, nil
	default:
		return "", fmt.Errorf("unsupported skill install profile %q", profile)
	}
}

func skillInstallMetadataByPublicName() map[string]skillInstallMetadata {
	out := map[string]skillInstallMetadata{}
	add := func(meta skillInstallMetadata) {
		meta.PublicName = strings.TrimSpace(meta.PublicName)
		meta.SourceID = strings.TrimSpace(meta.SourceID)
		if meta.PublicName == "" || meta.SourceID == "" {
			panic("toolgen: skill install metadata has empty identity")
		}
		meta.Profiles = uniqueInstallProfiles(meta.Profiles)
		meta.Requires = uniqueStrings(meta.Requires)
		out[meta.PublicName] = meta
	}

	for _, id := range governanceSurfaceIDs(func(governanceSurfaceDescriptor) bool { return true }) {
		add(skillInstallMetadata{
			PublicName:    adapterSkillName(id),
			SourceID:      id,
			Kind:          generatedSkillKindHost,
			Profiles:      []SkillInstallProfile{SkillInstallProfileFull},
			Requires:      installRequiresByPublicName[adapterSkillName(id)],
			AlwaysInstall: true,
		})
	}
	for _, id := range standaloneNames {
		add(skillInstallMetadata{
			PublicName:    adapterSkillName(id),
			SourceID:      id,
			Kind:          generatedSkillKindHost,
			Profiles:      []SkillInstallProfile{SkillInstallProfileFull},
			Requires:      installRequiresByPublicName[adapterSkillName(id)],
			AlwaysInstall: id == workflowSkillID,
		})
	}
	for _, id := range techniqueNames {
		if !shouldExportAsHostSkill(id) {
			continue
		}
		add(skillInstallMetadata{
			PublicName: adapterSkillName(id),
			SourceID:   id,
			Kind:       generatedSkillKindHost,
			Profiles:   []SkillInstallProfile{SkillInstallProfileFull},
			Requires:   installRequiresByPublicName[adapterSkillName(id)],
		})
	}
	for _, id := range catalogSkillIDs {
		if isGovernanceSurfaceID(id) || !shouldExportAsHostSkill(id) {
			continue
		}
		add(skillInstallMetadata{
			PublicName: adapterSkillName(id),
			SourceID:   id,
			Kind:       generatedSkillKindHost,
			Profiles:   []SkillInstallProfile{SkillInstallProfileFull},
			Requires:   installRequiresByPublicName[adapterSkillName(id)],
		})
	}
	for _, id := range commandIDs() {
		add(skillInstallMetadata{
			PublicName:    adapterSkillName(id),
			SourceID:      id,
			Kind:          generatedSkillKindCommand,
			Profiles:      []SkillInstallProfile{SkillInstallProfileFull},
			Requires:      installRequiresByPublicName[adapterSkillName(id)],
			AlwaysInstall: slices.Contains(alwaysInstalledCommandIDs, id),
		})
	}
	for _, def := range namespaceRouterDefinitions {
		add(skillInstallMetadata{
			PublicName: adapterSkillName(def.ID),
			SourceID:   def.ID,
			Kind:       generatedSkillKindRouter,
			Profiles:   []SkillInstallProfile{SkillInstallProfileCore},
			Requires:   []string{adapterSkillName(workflowSkillID)},
		})
	}
	return out
}

func uniqueInstallProfiles(in []SkillInstallProfile) []SkillInstallProfile {
	seen := map[SkillInstallProfile]struct{}{}
	out := []SkillInstallProfile{}
	for _, profile := range in {
		normalized, err := normalizeSkillInstallProfile(profile)
		if err != nil {
			panic(err)
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	slices.Sort(out)
	return out
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range in {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func (c skillInstallClosure) includesHostSkill(id string) bool {
	_, ok := c.hostIDs[strings.TrimSpace(id)]
	return ok
}

func (c skillInstallClosure) includesCommandSkill(id string) bool {
	_, ok := c.commandIDs[strings.TrimSpace(id)]
	return ok
}

func (c skillInstallClosure) routerDefinitions() []namespaceRouterDefinition {
	out := []namespaceRouterDefinition{}
	for _, def := range namespaceRouterDefinitions {
		if _, ok := c.routerIDs[def.ID]; ok {
			out = append(out, def)
		}
	}
	return out
}

func allGeneratedSkillDirNameSet(cfg ToolConfig) map[string]struct{} {
	managed := map[string]struct{}{}
	for _, names := range [][]string{
		GovernanceSkillNames,
		standaloneGovernanceNames,
		TemplatedGovernanceSkillNames,
		standaloneNames,
		techniqueNames,
		catalogSkillIDs,
	} {
		for _, name := range names {
			managed[adapterSkillName(name)] = struct{}{}
		}
	}
	for _, router := range namespaceRouterDefinitions {
		managed[adapterSkillName(router.ID)] = struct{}{}
	}
	if cfg.CommandSkillSurface {
		for _, name := range commandSkillDirNames() {
			managed[name] = struct{}{}
		}
	}
	return managed
}
