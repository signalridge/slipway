package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrimarySurfaceForCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		command    string
		publicName string
		backingID  string
	}{
		{command: "review", publicName: "independent-review", backingID: "independent-review"},
		{command: "fix", publicName: "root-cause-tracing", backingID: "root-cause-tracing"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.command, func(t *testing.T) {
			rec, ok := PrimaryForCommand(tc.command)
			require.True(t, ok, "expected primary surface for %s", tc.command)
			assert.Equal(t, tc.publicName, rec.PublicName)
			assert.Equal(t, tc.backingID, rec.BackingID)
			assert.Equal(t, SurfacePrimary, rec.Class)
		})
	}
}

func TestChangeScopedPrimaryViewForCommand(t *testing.T) {
	t.Parallel()

	// health retains its primary route; status no longer has one.
	t.Run("health", func(t *testing.T) {
		rec, ok := PrimaryForCommand("health")
		require.True(t, ok, "expected primary view for health")
		assert.Equal(t, "incident", rec.PublicName)
		assert.Equal(t, "incident-response", rec.BackingID)
		assert.Equal(t, SurfacePrimary, rec.Class)
		assert.Contains(t, rec.Summary, "Change-scoped")
	})

	t.Run("status_no_primary", func(t *testing.T) {
		_, ok := PrimaryForCommand("status")
		assert.False(t, ok, "status must not have a primary route")
	})

	t.Run("validate_no_primary", func(t *testing.T) {
		_, ok := PrimaryForCommand("validate")
		assert.False(t, ok, "validate must not have a primary route")
	})
}

func TestExplicitFocusRegistryPerCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		command string
		want    []string
	}{
		{command: "review", want: []string{"calibration", "sast"}},
		{command: "validate", want: []string{"mutation", "property", "sast", "spec-trace"}},
		// repair intentionally exposes NO explicit focus (issue #88): it never
		// runs external scanners, so the false `sast` focus was removed.
		{command: "repair", want: []string{}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.command, func(t *testing.T) {
			records := ExplicitFocusesForCommand(tc.command)
			got := make([]string, 0, len(records))
			for _, rec := range records {
				got = append(got, rec.PublicName)
				assert.Equal(t, SurfaceExplicitFocus, rec.Class)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSurfacePolicyBackingsResolveToRegisteredSkills(t *testing.T) {
	t.Parallel()

	reg := DefaultRegistry()
	for _, rec := range surfacePolicy {
		rec := rec
		t.Run(rec.Command+"-"+rec.PublicName, func(t *testing.T) {
			_, ok := reg.Lookup(rec.BackingID)
			assert.Truef(t, ok, "surface backing %q must resolve in registry", rec.BackingID)
		})
	}
}

func TestCalibrationHostAttachmentSurvivesFocusMigration(t *testing.T) {
	t.Parallel()

	res := Resolve(DefaultRegistry(), Signals{Host: "code-quality-review"})
	require.NotEmpty(t, res.Supports)

	found := false
	for _, support := range res.Supports {
		if support.SkillID == "multi-reviewer-calibration" {
			found = true
			assert.Equal(t, AttachmentProcedure, support.Kind)
		}
	}
	assert.True(t, found, "multi-reviewer-calibration must stay attached on code-quality-review host")

	for _, key := range res.HydrateReferences {
		assert.NotContains(t, key, "multi-reviewer-calibration/",
			"focus migration must not leak calibration hydrate on the host path")
	}
}

func TestCommandScopedBindingsDoNotAutoPopulateSupports(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"review", "validate", "repair"} {
		command := command
		t.Run(command, func(t *testing.T) {
			res := Resolve(DefaultRegistry(), Signals{Command: command})
			assert.Empty(t, res.Supports,
				"command-only resolution for %s must not auto-populate supports from command-scoped bindings", command)
		})
	}
}
