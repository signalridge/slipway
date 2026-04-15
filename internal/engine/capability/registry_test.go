package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRegistryLoadsFoundationSkills(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()

	want := []string{
		"ci-triage",
		"context-assembly",
		"coverage-analysis",
		"fresh-verification-evidence",
		"gha-security-review",
		"git-recovery",
		"incident-response",
		"independent-review",
		"multi-reviewer-calibration",
		"mutation-testing",
		"parallel-executor-contract",
		"performance-profiling",
		"plan-authoring",
		"property-testing",
		"review-comment-triage",
		"root-cause-tracing",
		"sast-orchestration",
		"scope-clarification",
		"security-review",
		"spec-trace",
		"supply-chain-audit",
		"tdd-proof",
		"threat-modeling",
		"variant-analysis",
	}
	assert.Equal(t, want, reg.IDs())
	assert.Equal(t, len(want), reg.Len())
}

func TestRegistryRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()
	sk := scopeClarification()
	_, err := NewRegistry(sk, sk)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate skill id")
}

func TestRegistryRejectsInvalidSkill(t *testing.T) {
	t.Parallel()
	sk := scopeClarification()
	sk.Tier = "T9"
	_, err := NewRegistry(sk)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tier")
}

func TestDefaultSkillsAllValid(t *testing.T) {
	t.Parallel()
	for _, sk := range defaultSkills() {
		assert.NoError(t, validateSkill(sk), sk.ID)
	}
}

func TestLookupReturnsRegisteredSkill(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	sk, ok := reg.Lookup("plan-authoring")
	require.True(t, ok)
	assert.Equal(t, DomainIntake, sk.Domain)
	assert.Equal(t, TierT1, sk.Tier)
}

func TestLookupMissingIsFalse(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	_, ok := reg.Lookup("unknown-skill")
	assert.False(t, ok)
}
