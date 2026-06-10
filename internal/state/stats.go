package state

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/model"
)

const codebaseMapFreshnessThreshold = 30 * 24 * time.Hour

var repoCodebaseMapDocs = []string{
	"STACK.md",
	"INTEGRATIONS.md",
	"ARCHITECTURE.md",
	"STRUCTURE.md",
	"CONVENTIONS.md",
	"TESTING.md",
	"CONCERNS.md",
}

type CodebaseMapStats struct {
	Dir              string    `json:"dir" yaml:"dir"`
	PresentDocs      int       `json:"present_docs" yaml:"present_docs"`
	PopulatedDocs    int       `json:"populated_docs" yaml:"populated_docs"`
	MissingDocs      []string  `json:"missing_docs,omitempty" yaml:"missing_docs,omitempty"`
	ScaffoldOnlyDocs []string  `json:"scaffold_only_docs,omitempty" yaml:"scaffold_only_docs,omitempty"`
	LatestModifiedAt time.Time `json:"latest_modified_at,omitempty" yaml:"latest_modified_at,omitempty"`
	Freshness        string    `json:"freshness" yaml:"freshness"`
}

type RepoStats struct {
	ActiveChanges    []model.Change    `json:"active_changes" yaml:"active_changes"`
	ChangeLoadIssues []ChangeLoadIssue `json:"change_load_issues,omitempty" yaml:"change_load_issues,omitempty"`
	ArchiveCount     int               `json:"archive_count" yaml:"archive_count"`
	CodebaseMap      CodebaseMapStats  `json:"codebase_map" yaml:"codebase_map"`
}

func CollectRepoStats(root string, now time.Time) (RepoStats, error) {
	changes, issues, err := ListRepoChangesBestEffortWithIssues(root)
	if err != nil {
		return RepoStats{}, err
	}
	active := make([]model.Change, 0, len(changes))
	for _, change := range changes {
		if change.Status == model.ChangeStatusActive {
			active = append(active, change)
		}
	}

	archives, err := ListArchivedChangeSlugs(root)
	if err != nil {
		return RepoStats{}, err
	}

	codebaseMap, err := collectCodebaseMapStats(root, now)
	if err != nil {
		return RepoStats{}, err
	}

	return RepoStats{
		ActiveChanges:    active,
		ChangeLoadIssues: issues,
		ArchiveCount:     len(archives),
		CodebaseMap:      codebaseMap,
	}, nil
}

func collectCodebaseMapStats(root string, now time.Time) (CodebaseMapStats, error) {
	dir := CodebaseMapDir(root)
	stats := CodebaseMapStats{
		Dir:       DisplayPath(root, dir),
		Freshness: "missing",
	}

	latest := time.Time{}
	for _, name := range repoCodebaseMapDocs {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				stats.MissingDocs = append(stats.MissingDocs, name)
				continue
			}
			return CodebaseMapStats{}, err
		}
		stats.PresentDocs++
		data, err := os.ReadFile(path) // #nosec G304 -- path is resolved from Slipway state/governance authority before this read.
		if err != nil {
			return CodebaseMapStats{}, err
		}
		if codebaseMapDocIsScaffoldOnly(data) {
			stats.ScaffoldOnlyDocs = append(stats.ScaffoldOnlyDocs, name)
		} else {
			stats.PopulatedDocs++
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}

	stats.LatestModifiedAt = latest.UTC()
	switch {
	case stats.PresentDocs == 0:
		stats.Freshness = "missing"
	case len(stats.MissingDocs) > 0:
		stats.Freshness = "partial"
	case stats.PopulatedDocs == 0 && len(stats.ScaffoldOnlyDocs) > 0:
		stats.Freshness = "scaffold_only"
	case len(stats.ScaffoldOnlyDocs) > 0:
		stats.Freshness = "partial"
	case latest.IsZero():
		stats.Freshness = "unknown"
	case now.Sub(latest) > codebaseMapFreshnessThreshold:
		stats.Freshness = "stale"
	default:
		stats.Freshness = "fresh"
	}
	return stats, nil
}

func codebaseMapDocIsScaffoldOnly(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "<!--") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			item := strings.TrimSpace(trimmed[2:])
			if idx := strings.Index(item, ":"); idx >= 0 && strings.TrimSpace(item[idx+1:]) == "" {
				continue
			}
			return false
		}
		return false
	}
	return true
}
