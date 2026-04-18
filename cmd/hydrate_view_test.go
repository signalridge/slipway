package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// TestReviewTextRendersHydrateLine verifies that review text rendering keeps
// the explicit manual-review hydrate keys for the real Wave-1 review surface.
func TestReviewTextRendersHydrateLine(t *testing.T) {
	t.Parallel()
	view := reviewView{
		Slug:              "demo",
		CurrentState:      "S3_REVIEW",
		Verdict:           "pass",
		Mode:              "gha-security-review",
		HydrateReferences: []string{"gha-security-review/comment-triggered-commands.md", "gha-security-review/pwn-request.md"},
	}
	var buf bytes.Buffer
	if err := writeReviewText(&buf, view); err != nil {
		t.Fatalf("writeReviewText: %v", err)
	}
	out := buf.String()
	wantLine := "Hydrate: gha-security-review/comment-triggered-commands.md, gha-security-review/pwn-request.md"
	if !strings.Contains(out, wantLine) {
		t.Fatalf("review text missing hydrate line %q; got:\n%s", wantLine, out)
	}
	if n := strings.Count(out, "\nHydrate:"); n != 1 {
		t.Fatalf("expected exactly one Hydrate: line, saw %d; got:\n%s", n, out)
	}
}

// TestStatusTextRendersHydrateLine verifies the change-selected incident view
// renders the real incident-response hydrate keys.
func TestStatusTextRendersHydrateLine(t *testing.T) {
	t.Parallel()
	view := statusView{
		ExecutionMode:     "governed",
		Slug:              "demo",
		Phase:             "execution",
		LifecycleStatus:   "active",
		CurrentState:      "S2_EXECUTE",
		Mode:              "incident-response",
		EvidenceFreshness: "fresh",
		HydrateReferences: []string{"incident-response/incident-response-framework.md", "incident-response/incident-severity-matrix.md"},
	}
	rendered := renderStatusText(view)
	wantLine := "Hydrate: incident-response/incident-response-framework.md, incident-response/incident-severity-matrix.md"
	if !strings.Contains(rendered, wantLine) {
		t.Fatalf("status text missing hydrate line %q; got:\n%s", wantLine, rendered)
	}
	if !strings.Contains(rendered, "Focus: incident-response") {
		t.Fatalf("status text missing Focus label; got:\n%s", rendered)
	}
}

// TestStatusDiagnosticsRendersHydrateLine verifies hydrate rendering on the
// diagnostics-mode explicit-view path with a real manual status skill.
func TestStatusDiagnosticsRendersHydrateLine(t *testing.T) {
	t.Parallel()
	view := statusView{
		ExecutionMode:     "diagnostics",
		Mode:              "supply-chain-audit",
		EvidenceFreshness: "unknown",
		Diagnostics:       []string{"no active change"},
		HydrateReferences: []string{"supply-chain-audit/results-template.md"},
	}
	rendered := renderStatusText(view)
	if !strings.Contains(rendered, "Hydrate: supply-chain-audit/results-template.md") {
		t.Fatalf("diagnostics status text missing hydrate line; got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Focus: supply-chain-audit") {
		t.Fatalf("diagnostics status text missing Focus label; got:\n%s", rendered)
	}
}

func TestHealthTextUsesFocusLabel(t *testing.T) {
	t.Parallel()
	view := healthView{
		ExecutionMode:     "diagnostics",
		Mode:              "incident",
		HydrateReferences: []string{"incident-response/incident-severity-matrix.md"},
	}
	var buf bytes.Buffer
	if err := writeHealthText(&buf, view); err != nil {
		t.Fatalf("writeHealthText: %v", err)
	}
	rendered := buf.String()
	if !strings.Contains(rendered, "Focus: incident") {
		t.Fatalf("health text missing Focus label; got:\n%s", rendered)
	}
	if strings.Contains(rendered, "View: incident") {
		t.Fatalf("health text should not render View label anymore; got:\n%s", rendered)
	}
}

// TestNextTextRendersSupportHydrateOnly verifies the next text surface keeps
// hydrate references scoped to support hints instead of inventing a top-level
// next-skill hydrate line.
func TestNextTextRendersSupportHydrateOnly(t *testing.T) {
	t.Parallel()
	view := nextView{
		Slug:            "demo",
		Phase:           "execution",
		LifecycleStatus: "active",
		ExecutionMode:   "governed",
		CurrentState:    "S2_EXECUTE",
		NextSkill: &nextSkillView{
			Name:            "security-review",
			PromptPath:      ".slipway/skills/security-review/SKILL.md",
			VerificationDir: ".slipway/changes/demo/security-review/",
			State:           "pending",
			TechniqueHints: []techniqueHint{
				{
					Name:              "skill:root-cause-tracing",
					Reason:            "[support] investigate regression",
					HydrateReferences: []string{"root-cause-tracing/hypothesis-testing.md", "root-cause-tracing/root-cause-tracing.md"},
				},
			},
		},
	}
	var buf bytes.Buffer
	if err := writeNextHuman(&buf, view); err != nil {
		t.Fatalf("writeNextHuman: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "  Hydrate: security-review/") {
		t.Fatalf("next text should not render a top-level next-skill hydrate line; got:\n%s", out)
	}
	if !strings.Contains(out, "    Hydrate: root-cause-tracing/hypothesis-testing.md, root-cause-tracing/root-cause-tracing.md") {
		t.Fatalf("next text missing support hint hydrate line; got:\n%s", out)
	}
}

// TestNormalizeHydrateKeysDedupesAndSorts locks the surface contract.
func TestNormalizeHydrateKeysDedupesAndSorts(t *testing.T) {
	t.Parallel()
	got := normalizeHydrateKeys([]string{
		"security-review/xss.md",
		"security-review/authentication.md",
		"security-review/xss.md",
		"  ",
		"security-review/injection.md",
	})
	want := []string{
		"security-review/authentication.md",
		"security-review/injection.md",
		"security-review/xss.md",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d keys, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d: want %q got %q (full %v)", i, want[i], got[i], got)
		}
	}
}
