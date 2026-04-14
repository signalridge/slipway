package capability

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestBySourceCoverageMatchesProvenance is the B6 provenance-coverage gate.
// It parses docs/distillation/by-source.md and cross-checks every
// `standalone` or `partial-only` row against the provenance.yaml files of
// shipped catalog skills. Sources in other disposition buckets (absorbed,
// posture-only, view-only, route-only, deferred) are not required to have
// a provenance node and are allowed.
func TestBySourceCoverageMatchesProvenance(t *testing.T) {
	t.Parallel()

	bySourcePath := filepath.Join("..", "..", "..", "docs", "distillation", "by-source.md")
	raw, err := os.ReadFile(bySourcePath)
	require.NoError(t, err)

	rows := parseBySourceRows(string(raw))
	require.NotEmpty(t, rows, "by-source.md parsed no rows")

	provenanceSources := loadAllProvenanceSources(t)

	for _, row := range rows {
		row := row
		if row.disposition != "standalone" && row.disposition != "partial-only" {
			continue
		}
		t.Run(row.source, func(t *testing.T) {
			t.Parallel()
			_, present := provenanceSources[row.source]
			assert.True(t, present,
				"%s: disposition=%s but source not found in any provenance.yaml",
				row.source, row.disposition)
		})
	}
}

type bySourceRow struct {
	source      string
	disposition string
	target      string
	batch       string
}

var bySourceRowRe = regexp.MustCompile(`^\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|$`)

func parseBySourceRows(content string) []bySourceRow {
	var rows []bySourceRow
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimRight(raw, "\r")
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		m := bySourceRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		source := m[1]
		// Skip header and separator rows.
		if source == "Source" || strings.HasPrefix(source, "---") {
			continue
		}
		// Keep only source-corpus entries of the shape <vendor>/<skill>.
		// This naturally skips coverage-snapshot rows and other non-source tables.
		if !strings.Contains(source, "/") {
			continue
		}
		rows = append(rows, bySourceRow{
			source:      source,
			disposition: m[2],
			target:      m[3],
			batch:       m[4],
		})
	}
	return rows
}

func loadAllProvenanceSources(t *testing.T) map[string]struct{} {
	t.Helper()
	root := skillsDir(t)
	reg := DefaultRegistry()
	out := make(map[string]struct{})
	for _, sk := range reg.All() {
		path := filepath.Join(root, sk.ID, "provenance.yaml")
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		var p struct {
			Sources []struct {
				Source string `yaml:"source"`
			} `yaml:"sources"`
		}
		require.NoError(t, yaml.Unmarshal(raw, &p))
		for _, s := range p.Sources {
			out[strings.TrimSpace(s.Source)] = struct{}{}
		}
	}
	return out
}

// TestProvenanceSourcesAppearInBySource asserts the reverse direction: every
// source referenced by a provenance.yaml must appear as a row in by-source.md
// so the two documents cannot silently drift.
func TestProvenanceSourcesAppearInBySource(t *testing.T) {
	t.Parallel()

	bySourcePath := filepath.Join("..", "..", "..", "docs", "distillation", "by-source.md")
	raw, err := os.ReadFile(bySourcePath)
	require.NoError(t, err)
	rows := parseBySourceRows(string(raw))

	rowIDs := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		rowIDs[r.source] = struct{}{}
	}

	sources := loadAllProvenanceSources(t)
	ids := make([]string, 0, len(sources))
	for s := range sources {
		ids = append(ids, s)
	}
	sort.Strings(ids)
	for _, s := range ids {
		_, ok := rowIDs[s]
		assert.Truef(t, ok, "provenance references %q but no by-source.md row exists", s)
	}
}
