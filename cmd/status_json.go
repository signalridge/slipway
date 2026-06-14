package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
)

type governanceSummaryView struct {
	BlockedBy        []string               `json:"blocked_by,omitempty"`
	RequiredActions  []string               `json:"required_actions,omitempty"`
	SatisfiedActions []governanceActionView `json:"satisfied_actions,omitempty"`
	AuthorityRefs    []string               `json:"authority_refs,omitempty"`
}

type statusJSONView struct {
	view              statusView
	governanceSummary *governanceSummaryView
}

func buildStatusJSONResponse(view statusView) statusJSONView {
	return statusJSONView{
		view:              view,
		governanceSummary: buildGovernanceSummaryView(view),
	}
}

func (view statusJSONView) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(view.view)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	delete(payload, "governance_signals")
	delete(payload, "active_controls")
	delete(payload, "required_actions")
	if view.governanceSummary != nil {
		payload["governance_summary"] = view.governanceSummary
	}
	return json.Marshal(payload)
}

func buildGovernanceSummaryView(view statusView) *governanceSummaryView {
	var blockedBy []string
	var requiredActions []string
	var satisfiedActions []governanceActionView
	seenControls := map[string]struct{}{}
	seenActions := map[string]struct{}{}

	addControl := func(controlID string) {
		controlID = strings.TrimSpace(controlID)
		if controlID == "" {
			return
		}
		if _, ok := seenControls[controlID]; ok {
			return
		}
		seenControls[controlID] = struct{}{}
		blockedBy = append(blockedBy, controlID)
	}
	addAction := func(description string) {
		description = strings.TrimSpace(description)
		if description == "" {
			return
		}
		if _, ok := seenActions[description]; ok {
			return
		}
		seenActions[description] = struct{}{}
		requiredActions = append(requiredActions, description)
	}

	// required_actions is the full pending-obligation queue: every unsatisfied
	// action, advisory or blocking. It must NOT feed blocked_by. An unsatisfied
	// action only blocks when it is blocking-mode AND gates the current state —
	// a determination already made authoritatively by RequiredActionBlockers
	// (mode + state-scope filtered) and surfaced as governance_action_required
	// blockers below. Deriving blocked_by from raw unsatisfied actions here
	// reports advisory or not-yet-applicable controls (e.g. an advisory
	// independent-review at S2) as blockers, contradicting the real gate.
	for _, action := range view.RequiredActions {
		if action.Satisfied {
			if len(action.SatisfiedBy) > 0 {
				satisfiedActions = append(satisfiedActions, action)
			}
			continue
		}
		addAction(action.Description)
	}
	for _, blocker := range view.Blockers {
		if blocker.Code != "governance_action_required" {
			continue
		}
		controlID, description := governanceActionBlockerSummary(blocker.Detail)
		addControl(controlID)
		addAction(description)
	}

	if len(blockedBy) == 0 && len(requiredActions) == 0 && len(satisfiedActions) == 0 {
		return nil
	}

	summary := &governanceSummaryView{
		BlockedBy:        blockedBy,
		RequiredActions:  requiredActions,
		SatisfiedActions: satisfiedActions,
		AuthorityRefs:    governanceSummaryAuthorityRefs(view),
	}
	return summary
}

func governanceActionBlockerSummary(detail string) (string, string) {
	controlID, description, ok := strings.Cut(strings.TrimSpace(detail), ":")
	if !ok {
		return strings.TrimSpace(detail), ""
	}
	return strings.TrimSpace(controlID), strings.TrimSpace(description)
}

func governanceSummaryAuthorityRefs(view statusView) []string {
	var refs []string
	add := func(ref string) {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return
		}
		for _, existing := range refs {
			if existing == ref {
				return
			}
		}
		refs = append(refs, ref)
	}
	if view.SourceStateFile != "" {
		add(view.SourceStateFile)
	} else if view.Slug != "" {
		add(fmt.Sprintf("artifacts/changes/%s/change.yaml", view.Slug))
	}
	if view.Slug != "" {
		add(fmt.Sprintf("slipway health --governance --json --change %s", view.Slug))
	}
	return refs
}
