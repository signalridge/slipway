package capability

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvenanceValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      Provenance
		wantErr string
	}{
		{
			name: "standalone with extracted is valid",
			in: Provenance{Sources: []ProvenanceSource{{
				Source:     "superpowers/brainstorming",
				AbsorbedAs: AbsorbedStandalone,
				Extracted:  []string{"trigger-oriented posture"},
			}}},
		},
		{
			name: "invalid absorbed_as rejected",
			in: Provenance{Sources: []ProvenanceSource{{
				Source:     "a/b",
				AbsorbedAs: "sideways",
				Extracted:  []string{"x"},
			}}},
			wantErr: "invalid absorbed_as",
		},
		{
			name: "empty source rejected",
			in: Provenance{Sources: []ProvenanceSource{{
				AbsorbedAs: AbsorbedStandalone,
				Extracted:  []string{"x"},
			}}},
			wantErr: "empty source",
		},
		{
			name: "duplicate source rejected",
			in: Provenance{Sources: []ProvenanceSource{
				{Source: "a/b", AbsorbedAs: AbsorbedStandalone, Extracted: []string{"x"}},
				{Source: "a/b", AbsorbedAs: AbsorbedStandalone, Extracted: []string{"y"}},
			}},
			wantErr: "duplicate source",
		},
		{
			name: "empty extracted/dropped/conflicts rejected",
			in: Provenance{Sources: []ProvenanceSource{{
				Source:     "a/b",
				AbsorbedAs: AbsorbedStandalone,
			}}},
			wantErr: "must list at least one",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.in.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestLoadProvenanceReadsDisk(t *testing.T) {
	t.Parallel()
	// Pick any B1 skill; provenance.yaml must exist and parse.
	root := filepath.Join("..", "..", "..", "internal", "tmpl", "templates", "skills",
		"scope-clarification", "provenance.yaml")
	p, err := LoadProvenance(root)
	require.NoError(t, err)
	assert.NotEmpty(t, p.Sources)
	assert.NotEmpty(t, p.Coverage())
}
