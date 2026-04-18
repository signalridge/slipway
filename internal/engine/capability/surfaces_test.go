package capability

import (
	"slices"
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
		{command: "validate", publicName: "spec-trace", backingID: "spec-trace"},
		{command: "repair", publicName: "root-cause-tracing", backingID: "root-cause-tracing"},
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

	for _, command := range []string{"status", "health"} {
		command := command
		t.Run(command, func(t *testing.T) {
			rec, ok := PrimaryForCommand(command)
			require.True(t, ok, "expected primary view for %s", command)
			assert.Equal(t, "incident", rec.PublicName)
			assert.Equal(t, "incident-response", rec.BackingID)
			assert.Equal(t, SurfacePrimary, rec.Class)
			assert.Contains(t, rec.Summary, "Change-scoped")
		})
	}
}

func TestExplicitFocusRegistryPerCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		command string
		want    []string
	}{
		{command: "review", want: []string{"calibration", "sast"}},
		{command: "validate", want: []string{"mutation", "property", "sast"}},
		{command: "repair", want: []string{"sast"}},
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
	for _, rec := range AllSurfaces() {
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

func TestAllSurfacesStableOrder(t *testing.T) {
	t.Parallel()

	surfaces := AllSurfaces()
	require.NotEmpty(t, surfaces)
	assert.True(t, slices.IsSortedFunc(surfaces, func(a, b SurfaceRecord) int {
		switch {
		case a.Command < b.Command:
			return -1
		case a.Command > b.Command:
			return 1
		case classOrder(a.Class) < classOrder(b.Class):
			return -1
		case classOrder(a.Class) > classOrder(b.Class):
			return 1
		case a.PublicName < b.PublicName:
			return -1
		case a.PublicName > b.PublicName:
			return 1
		default:
			return 0
		}
	}), "AllSurfaces must return stable command/class/name order")
}
