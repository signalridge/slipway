package capability

import (
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/tmpl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIndependentReviewPreservesDiffOnlyRules locks the essential diff-scoped
// rules absorbed from the retired `differential-review` skill. Route-surface
// plan §8 requires the absorbed SKILL.md to still classify findings as new /
// pre-existing / worsened and to document the diff-scoped blocker policy.
func TestIndependentReviewPreservesDiffOnlyRules(t *testing.T) {
	t.Parallel()

	body, err := tmpl.Content("skills/independent-review/SKILL.md")
	require.NoError(t, err)

	lowered := strings.ToLower(body)
	for _, token := range []string{"new", "pre-existing", "worsened"} {
		assert.Contains(t, lowered, token,
			"independent-review SKILL.md must preserve the %q finding classification from differential-review", token)
	}
	assert.Contains(t, lowered, "diff-scoped blocker policy",
		"independent-review SKILL.md must preserve the diff-scoped blocker policy section")
}

// TestIndependentReviewPreservesDifferentialReviewEvidenceVerdictContract
// prevents the absorption from silently weakening the evidence contract from
// `verdict` to `artifact`. The registry entry and the absorbed SKILL.md both
// must keep the verdict-shaped contract visible.
func TestIndependentReviewPreservesDifferentialReviewEvidenceVerdictContract(t *testing.T) {
	t.Parallel()

	reg := DefaultRegistry()
	sk, ok := reg.Lookup("independent-review")
	require.True(t, ok, "independent-review must stay in the registry")
	assert.Equal(t, EvidenceVerdict, sk.Evidence,
		"independent-review must keep the verdict evidence contract after absorbing differential-review")

	body, err := tmpl.Content("skills/independent-review/SKILL.md")
	require.NoError(t, err)
	lowered := strings.ToLower(body)
	assert.Contains(t, lowered, "verdict",
		"absorbed SKILL.md must still speak in terms of the verdict evidence contract")
	assert.Contains(t, lowered, "evidence contract",
		"SKILL.md must keep an explicit evidence-contract section")
}

// TestIndependentReviewWithoutDiffContextKeepsBaseReviewContract ensures the
// absorbed diff-only section cannot leak its obligations into `review --all`
// or any other full-review execution path. The SKILL.md must explicitly gate
// the diff-scoped rules on a concrete diff target being in scope.
func TestIndependentReviewWithoutDiffContextKeepsBaseReviewContract(t *testing.T) {
	t.Parallel()

	body, err := tmpl.Content("skills/independent-review/SKILL.md")
	require.NoError(t, err)
	lowered := strings.ToLower(body)

	assert.Contains(t, lowered, "when a concrete diff target is in scope",
		"SKILL.md must gate diff-scoped rules on a concrete diff target")
	assert.Contains(t, lowered, "when no diff context is present",
		"SKILL.md must explicitly describe the no-diff fallback path")
	assert.Contains(t, lowered, "--all",
		"SKILL.md should mention `review --all` as the full-review path that skips diff-scoped rules")
}

// TestDifferentialReviewAbsentFromRegistry guards the hard-cut from
// route-surface plan §8: the registry must not carry `differential-review`
// after absorption, and it must not be admitted on any routed surface.
func TestDifferentialReviewAbsentFromRegistry(t *testing.T) {
	t.Parallel()

	reg := DefaultRegistry()
	_, ok := reg.Lookup("differential-review")
	assert.False(t, ok, "differential-review must not be in the registry after absorption")
	for _, id := range reg.IDs() {
		assert.NotEqual(t, "differential-review", id,
			"differential-review must not leak into DefaultRegistry().IDs()")
	}
}

// TestNonViewSkillsNotAdmittedOnStatusHealthViews regresses the §8 cleanup:
// supply-chain-audit / ci-triage / git-recovery / performance-profiling must
// not resolve as a `--view` alias on status or health after reclassification.
func TestNonViewSkillsNotAdmittedOnStatusHealthViews(t *testing.T) {
	t.Parallel()

	for _, alias := range []string{"supply-chain-audit", "ci-triage", "git-recovery", "performance-profiling"} {
		for _, cmd := range []string{"status", "health"} {
			_, ok := LookupView(cmd, alias)
			assert.False(t, ok, "%s must not be a valid --view alias on %s", alias, cmd)
		}
	}
}
