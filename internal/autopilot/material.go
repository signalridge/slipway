package autopilot

import (
	"errors"
	"fmt"

	"github.com/signalridge/slipway/internal/runstore"
)

// ActionMaterial is the versioned result of one local chapter read.
type ActionMaterial struct {
	ContractVersion      int                   `json:"contract_version"`
	MessageType          string                `json:"message_type"`
	RunID                string                `json:"run_id"`
	ActionID             string                `json:"action_id"`
	RequirementsRevision string                `json:"requirements_revision"`
	Section              ActionMaterialSection `json:"section"`
}

// ActionMaterialSection contains exactly one pinned normative chapter.
type ActionMaterialSection struct {
	Key             string            `json:"key"`
	Role            SourceSectionRole `json:"role"`
	Title           string            `json:"title"`
	SectionRevision string            `json:"section_revision"`
	Markdown        string            `json:"markdown"`
}

// ReadActionMaterial reads one locally pinned chapter without network access.
func (service *Service) ReadActionMaterial(
	runID,
	actionID,
	sectionKey string,
) (ActionMaterial, error) {
	if !validSourceSectionKey(sectionKey) {
		return ActionMaterial{}, &ProtocolError{
			Code:    "material_section_invalid",
			Message: "section must be a valid source section key",
			Next:    NoneNext(service.openIdentity.ID),
		}
	}
	if _, err := service.validateOpenWorkspace(); err != nil {
		return ActionMaterial{}, err
	}

	var run Run
	var section ActionRequirementSection
	var requirementsRevision string
	var data []byte
	err := service.store.VisitWithMaterialReader(
		runID,
		func(event runstore.Event) error {
			return applyRunEvent(&run, event)
		},
		func(readMaterial runstore.MaterialReader) error {
			if run.ID != runID {
				return errors.New("run journal identity mismatch")
			}
			if err := service.validateRunWorkspace(run); err != nil {
				return err
			}
			record := findActionRecord(&run, actionID)
			if record == nil {
				return &ProtocolError{
					Code:    "material_action_not_found",
					Message: fmt.Sprintf("action %q does not belong to run %q", actionID, runID),
					Next:    NoneNext(run.WorkspaceIdentity.ID),
				}
			}
			if run.State == RunStopped || run.State == RunEnded || record.Voided ||
				(record.Outcome != nil && record.Outcome.Status != OutcomeNeedsInput) ||
				run.CurrentAction == nil || run.CurrentAction.ActionID != actionID {
				return &ProtocolError{
					Code:    "material_action_stale",
					Message: "only the current non-void action may read material for execution",
					Next:    mustDeriveNext(run),
				}
			}
			if record.Action.Source == nil || record.Action.Requirements == nil {
				return &ProtocolError{
					Code:    "material_unavailable",
					Message: "ad-hoc action has no source material",
					Next:    NoneNext(run.WorkspaceIdentity.ID),
				}
			}

			found := false
			for _, candidate := range record.Action.Requirements.Sections {
				if candidate.Key == sectionKey {
					section = candidate
					found = true
					break
				}
			}
			if !found {
				return &ProtocolError{
					Code:    "material_section_not_found",
					Message: fmt.Sprintf("section %q is not available to action %q", sectionKey, actionID),
					Next:    NoneNext(run.WorkspaceIdentity.ID),
				}
			}

			digest := section.MaterialSHA256
			if !validSHA256(digest) {
				return &ProtocolError{
					Code:    "material_unavailable",
					Message: "action section has no valid pinned material reference",
					Next:    NoneNext(run.WorkspaceIdentity.ID),
				}
			}
			read, err := readMaterial(digest)
			if err != nil {
				return &ProtocolError{
					Code:    "material_unavailable",
					Message: "pinned material cannot be read or verified: " + err.Error(),
					Next:    NoneNext(run.WorkspaceIdentity.ID),
				}
			}
			data = read
			requirementsRevision = record.Action.Requirements.RequirementsRevision
			return nil
		},
	)
	if err != nil {
		return ActionMaterial{}, err
	}
	if len(data) != section.Bytes {
		return ActionMaterial{}, &ProtocolError{
			Code:    "material_corrupt",
			Message: "pinned material byte count does not match action catalog",
			Next:    NoneNext(run.WorkspaceIdentity.ID),
		}
	}
	markdown := string(data)
	if materialRevision(markdown) != section.MaterialSHA256 {
		return ActionMaterial{}, errors.New("material digest validation disagrees with runstore")
	}
	if sectionRevision(section.Key, section.Role, section.Title, markdown) != section.SectionRevision {
		return ActionMaterial{}, &ProtocolError{
			Code:    "material_corrupt",
			Message: "pinned material does not match the action section revision",
			Next:    NoneNext(run.WorkspaceIdentity.ID),
		}
	}
	return ActionMaterial{
		ContractVersion:      ContractVersion,
		MessageType:          "action_material",
		RunID:                runID,
		ActionID:             actionID,
		RequirementsRevision: requirementsRevision,
		Section: ActionMaterialSection{
			Key:             section.Key,
			Role:            section.Role,
			Title:           section.Title,
			SectionRevision: section.SectionRevision,
			Markdown:        markdown,
		},
	}, nil
}
