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
		"gha-security-review",
		"git-recovery",
		"incident-response",
		"independent-review",
		"multi-reviewer-calibration",
		"mutation-testing",
		"property-testing",
		"review-comment-triage",
		"root-cause-tracing",
		"sast-orchestration",
		"security-review",
		"spec-trace",
		"supply-chain-audit",
		"test-design",
		"threat-modeling",
		"variant-analysis",
	}
	assert.Equal(t, want, reg.IDs())
	assert.Equal(t, len(want), reg.Len())
}

func TestRegistryRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()
	sk := contextAssembly()
	_, err := NewRegistry(sk, sk)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate skill id")
}

func TestRegistryRejectsInvalidSkill(t *testing.T) {
	t.Parallel()
	sk := contextAssembly()
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
	sk, ok := reg.Lookup("independent-review")
	require.True(t, ok)
	assert.Equal(t, DomainReviewQuality, sk.Domain)
	assert.Equal(t, TierT1, sk.Tier)
}

func TestLookupMissingIsFalse(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	_, ok := reg.Lookup("unknown-skill")
	assert.False(t, ok)
}

func TestStatusSuggestedOnlySkillsStayOffStatusCommand(t *testing.T) {
	t.Parallel()

	reg := DefaultRegistry()
	for _, id := range []string{
		"supply-chain-audit",
		"ci-triage",
		"git-recovery",
	} {
		id := id
		t.Run(id, func(t *testing.T) {
			sk, ok := reg.Lookup(id)
			require.True(t, ok)

			for _, binding := range sk.Bindings {
				if binding.Type != BindingCommandAuto {
					continue
				}
				assert.NotEqual(t, "status", binding.Target, "status command binding must stay removed")
			}
		})
	}
}

func TestPromotedReviewSkillsKeepReviewCommandAutoWithoutHostEmbedding(t *testing.T) {
	t.Parallel()

	reg := DefaultRegistry()
	cases := []struct {
		id         string
		attachment AttachmentMode
	}{
		{id: "independent-review", attachment: AttachmentReportSchema},
		{id: "security-review", attachment: AttachmentChecklist},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()

			sk, ok := reg.Lookup(tc.id)
			require.True(t, ok)

			foundCommandAuto := false
			for _, binding := range sk.Bindings {
				assert.NotEqual(t, BindingHostEmbedded, binding.Type,
					"%s is a workflow-owned S3 host and must not be host-embedded into peer review hosts", tc.id)
				if binding.Type == BindingCommandAuto &&
					binding.Target == "review" &&
					binding.Attachment == tc.attachment {
					foundCommandAuto = true
				}
			}
			assert.True(t, foundCommandAuto, "%s must preserve review command-auto binding", tc.id)
		})
	}
}

func TestDefaultSkillsDoNotUseRemovedCommandViewBindings(t *testing.T) {
	t.Parallel()

	for _, sk := range defaultSkills() {
		for _, binding := range sk.Bindings {
			assert.NotEqual(
				t,
				"command-view",
				string(binding.Type),
				"skill %q still uses removed command-view binding",
				sk.ID,
			)
		}
	}
}

// TestDefaultRegistryReturnsMutationIsolatedSkills guards the mutation isolation
// that the sync.OnceValue memoization of DefaultRegistry could otherwise break:
// every caller shares one *Registry, so Lookup and All must return skills whose
// mutable slice fields are independent copies. Without that, a caller mutating a
// returned skill would silently poison every later lookup in the process.
// independent-review carries a Bindings entry plus a HostCapabilities entry with
// a nested FallbackModes slice, so it exercises the shallow and the nested field.
func TestDefaultRegistryReturnsMutationIsolatedSkills(t *testing.T) {
	const id = "independent-review"

	first, ok := DefaultRegistry().Lookup(id)
	require.True(t, ok)
	require.NotEmpty(t, first.Bindings, "fixture skill must expose a binding")
	require.NotEmpty(t, first.HostCapabilities, "fixture skill must expose a host capability")
	require.NotEmpty(t, first.HostCapabilities[0].FallbackModes, "fixture must expose nested fallback modes")

	wantTarget := first.Bindings[0].Target
	wantFallback := first.HostCapabilities[0].FallbackModes[0]

	// Mutate the copy returned by Lookup, including the nested FallbackModes.
	first.Bindings[0].Target = "__mutated_target__"
	first.HostCapabilities[0].FallbackModes[0] = "__mutated_fallback__"

	afterLookup, ok := DefaultRegistry().Lookup(id)
	require.True(t, ok)
	assert.Equal(t, wantTarget, afterLookup.Bindings[0].Target,
		"Lookup mutation leaked into the shared registry")
	assert.Equal(t, wantFallback, afterLookup.HostCapabilities[0].FallbackModes[0],
		"nested FallbackModes mutation via Lookup leaked into the shared registry")

	// All must isolate the same way.
	for _, sk := range DefaultRegistry().All() {
		if sk.ID != id {
			continue
		}
		sk.Bindings[0].Target = "__mutated_via_all__"
		sk.HostCapabilities[0].FallbackModes[0] = "__mutated_via_all__"
	}
	afterAll, ok := DefaultRegistry().Lookup(id)
	require.True(t, ok)
	assert.Equal(t, wantTarget, afterAll.Bindings[0].Target,
		"All mutation leaked into the shared registry")
	assert.Equal(t, wantFallback, afterAll.HostCapabilities[0].FallbackModes[0],
		"nested FallbackModes mutation via All leaked into the shared registry")
}
