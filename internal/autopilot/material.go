package autopilot

import (
	"errors"
	"fmt"

	"github.com/signalridge/slipway/internal/runstore"
)

// PinnedMaterial is the versioned result of reading one pinned chapter for
// inspection rather than execution. It deliberately carries no action_id: it
// reports what the Run has pinned, in any state, and authorizes nothing.
type PinnedMaterial struct {
	ContractVersion      int                   `json:"contract_version"`
	MessageType          string                `json:"message_type"`
	RunID                string                `json:"run_id"`
	RunState             RunState              `json:"run_state"`
	SourceRevision       string                `json:"source_revision"`
	RequirementsRevision string                `json:"requirements_revision"`
	Section              ActionMaterialSection `json:"section"`
}

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
			Next:    materialStatusNext(service.store.RepositoryRoot(), service.openIdentity.ID, runID),
		}
	}
	if _, err := service.validateOpenWorkspace(); err != nil {
		return ActionMaterial{}, err
	}
	if _, err := service.loadOwnedRunHeader(runID); err != nil {
		return ActionMaterial{}, service.normalizeRunLoadError(err)
	}

	var run Run
	var section ActionRequirementSection
	var requirementsRevision string
	var data []byte
	err := service.store.VisitWithMaterialReader(
		runID,
		func(event runstore.Event) error {
			return replayRunProjectionEvent(&run, event)
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
					Next:    materialRecoveryNext(run),
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
					Next:    materialRecoveryNext(run),
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
					Next:    materialRecoveryNext(run),
				}
			}

			digest := section.MaterialSHA256
			if !validSHA256(digest) {
				return &ProtocolError{
					Code:    "material_unavailable",
					Message: "action section has no valid pinned material reference",
					Next:    materialRecoveryNext(run),
				}
			}
			read, err := readMaterial(digest)
			if err != nil {
				return &ProtocolError{
					Code:    "material_unavailable",
					Message: "pinned material cannot be read or verified: " + err.Error(),
					Next:    materialRecoveryNext(run),
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
			Next:    materialRecoveryNext(run),
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
			Next:    materialRecoveryNext(run),
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

// ReadPinnedMaterial reads one currently pinned chapter for inspection. Unlike
// ReadActionMaterial it accepts any run state and needs no current Action: a
// user who stopped or ended a Run can still read exactly what it pinned, which
// the catalog in `status` identifies but does not contain. It takes no lock,
// writes nothing, and confers no execution or publication authority.
func (service *Service) ReadPinnedMaterial(runID, sectionKey string) (PinnedMaterial, error) {
	if !validSourceSectionKey(sectionKey) {
		return PinnedMaterial{}, &ProtocolError{
			Code:    "material_section_invalid",
			Message: "section must be a valid source section key",
			Next:    materialStatusNext(service.store.RepositoryRoot(), service.openIdentity.ID, runID),
		}
	}
	if _, err := service.validateOpenWorkspace(); err != nil {
		return PinnedMaterial{}, err
	}
	if _, err := service.loadOwnedRunHeader(runID); err != nil {
		return PinnedMaterial{}, service.normalizeRunLoadError(err)
	}

	var run Run
	var section PinnedSourceSection
	var data []byte
	err := service.store.VisitWithMaterialReader(
		runID,
		func(event runstore.Event) error {
			return replayRunProjectionEvent(&run, event)
		},
		func(readMaterial runstore.MaterialReader) error {
			if run.ID != runID {
				return errors.New("run journal identity mismatch")
			}
			if err := service.validateRunWorkspace(run); err != nil {
				return err
			}
			if run.PinnedSource == nil {
				return &ProtocolError{
					Code:    "material_unavailable",
					Message: "ad-hoc run has no pinned source",
					Next:    materialRecoveryNext(run),
				}
			}
			found := false
			for _, candidate := range run.PinnedSource.Sections {
				if candidate.Key == sectionKey {
					section = candidate
					found = true
					break
				}
			}
			if !found {
				return &ProtocolError{
					Code:    "material_section_not_found",
					Message: fmt.Sprintf("section %q is not pinned by run %q", sectionKey, runID),
					Next:    materialRecoveryNext(run),
				}
			}
			if !validSHA256(section.MaterialSHA256) {
				return &ProtocolError{
					Code:    "material_unavailable",
					Message: "pinned section has no valid material reference",
					Next:    materialRecoveryNext(run),
				}
			}
			read, err := readMaterial(section.MaterialSHA256)
			if err != nil {
				return &ProtocolError{
					Code:    "material_unavailable",
					Message: "pinned material cannot be read or verified: " + err.Error(),
					Next:    materialRecoveryNext(run),
				}
			}
			data = read
			return nil
		},
	)
	if err != nil {
		return PinnedMaterial{}, err
	}
	markdown := string(data)
	if err := verifyPinnedMaterial(run, section, markdown, len(data)); err != nil {
		return PinnedMaterial{}, err
	}
	return PinnedMaterial{
		ContractVersion:      ContractVersion,
		MessageType:          "pinned_material",
		RunID:                runID,
		RunState:             run.State,
		SourceRevision:       run.PinnedSource.SourceRevision,
		RequirementsRevision: run.PinnedSource.RequirementsRevision,
		Section: ActionMaterialSection{
			Key:             section.Key,
			Role:            section.Role,
			Title:           section.Title,
			SectionRevision: section.SectionRevision,
			Markdown:        markdown,
		},
	}, nil
}

// verifyPinnedMaterial repeats the execution path's byte, material, and section
// checks. Inspection reads the same bytes an Action would, so it must refuse to
// display material that no longer matches the catalog rather than presenting
// corrupt content as the pinned requirement.
func verifyPinnedMaterial(run Run, section PinnedSourceSection, markdown string, size int) error {
	if size != section.Bytes {
		return &ProtocolError{
			Code:    "material_corrupt",
			Message: "pinned material byte count does not match the source catalog",
			Next:    materialRecoveryNext(run),
		}
	}
	if materialRevision(markdown) != section.MaterialSHA256 {
		return errors.New("material digest validation disagrees with runstore")
	}
	if sectionRevision(section.Key, section.Role, section.Title, markdown) != section.SectionRevision {
		return &ProtocolError{
			Code:    "material_corrupt",
			Message: "pinned material does not match the pinned section revision",
			Next:    materialRecoveryNext(run),
		}
	}
	return nil
}

func materialRecoveryNext(run Run) Next {
	return materialStatusNext(run.Workspace, run.WorkspaceIdentity.ID, run.ID)
}

func materialStatusNext(root, workspaceIdentity, runID string) Next {
	return Next{
		Operation:         NextOperationCommand,
		WorkspaceIdentity: workspaceIdentity,
		workspaceRoot:     root,
		Variants: []NextVariant{{
			ID:       "inspect-run",
			BaseArgv: []string{"slipway", "status", runID, "--root", root},
			Inputs:   []NextInput{},
		}},
	}
}
