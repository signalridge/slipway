package cmd

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

type selectedPriorContextView struct {
	Slug            string   `json:"slug"`
	SourceStateFile string   `json:"source_state_file"`
	SelectedBecause []string `json:"selected_because,omitempty"`
}

type unresolvedDependencyView struct {
	Slug     string   `json:"slug"`
	Provides []string `json:"provides,omitempty"`
	Reason   string   `json:"reason"`
}

func buildSelectedPriorContext(root string, deps model.ContextDependencies) ([]selectedPriorContextView, []unresolvedDependencyView) {
	if len(deps.Requires) == 0 {
		return nil, nil
	}

	selected := make([]selectedPriorContextView, 0, len(deps.Requires))
	unresolved := make([]unresolvedDependencyView, 0)
	for _, req := range deps.Requires {
		archived, err := state.LoadArchivedChange(root, req.Slug)
		if err != nil {
			reason := "archive_invalid"
			if errors.Is(err, fs.ErrNotExist) {
				reason = "archive_not_found"
			}
			unresolved = append(unresolved, unresolvedDependencyView{
				Slug:     req.Slug,
				Provides: append([]string(nil), req.Provides...),
				Reason:   reason,
			})
			continue
		}
		because := make([]string, 0, len(req.Provides))
		for _, provide := range req.Provides {
			because = append(because, "requires:"+provide)
		}
		if len(because) == 0 {
			because = []string{"requires:" + req.Slug}
		}
		selected = append(selected, selectedPriorContextView{
			Slug:            archived.Slug,
			SourceStateFile: filepath.ToSlash(filepath.Join("artifacts", "changes", "archived", archived.Slug, "change.yaml")),
			SelectedBecause: because,
		})
	}

	slices.SortFunc(selected, func(a, b selectedPriorContextView) int {
		if a.Slug < b.Slug {
			return -1
		}
		if a.Slug > b.Slug {
			return 1
		}
		return 0
	})
	if len(selected) == 0 {
		selected = nil
	}
	slices.SortFunc(unresolved, func(a, b unresolvedDependencyView) int {
		if a.Slug < b.Slug {
			return -1
		}
		if a.Slug > b.Slug {
			return 1
		}
		return 0
	})
	if len(unresolved) == 0 {
		unresolved = nil
	}
	return selected, unresolved
}
