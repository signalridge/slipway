package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/signalridge/speclane/internal/engine/action"
	"github.com/signalridge/speclane/internal/engine/artifact"
	"github.com/signalridge/speclane/internal/engine/gate"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
)

type pivotOptions struct {
	kind string
}

type pivotView struct {
	RequestID    string   `json:"request_id"`
	Kind         string   `json:"kind"`
	LaneMode     string   `json:"lane_mode"`
	CurrentState string   `json:"current_state"`
	Level        string   `json:"level"`
	Blockers     []string `json:"blockers,omitempty"`
}

func newPivotCmd() *cobra.Command {
	opts := pivotOptions{}
	cmd := &cobra.Command{
		Use:   "pivot",
		Short: "Reroute or rescope an active request",
		RunE: func(cmd *cobra.Command, _ []string) error {
			kind := strings.ToLower(strings.TrimSpace(opts.kind))
			if kind == "" {
				kind = string(gate.PivotKindReroute)
			}
			if kind != string(gate.PivotKindReroute) && kind != string(gate.PivotKindRescope) {
				return fmt.Errorf("invalid --kind %q; expected reroute|rescope", opts.kind)
			}

			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			return withWorkspaceStateLock(root, "pivot", func() error {
				active, err := ensureRequestScopedActive(root)
				if err != nil {
					return err
				}

				var view pivotView
				switch active.Mode {
				case state.ActiveResolutionModeAdmissionOnly:
					if kind == string(gate.PivotKindRescope) {
						return fmt.Errorf("rescope is valid only for governed requests in S6")
					}
					admission, err := state.LoadAdmission(root, active.RequestID)
					if err != nil {
						return err
					}
					if !isPivotState(admission.CurrentState) {
						return fmt.Errorf("pivot is allowed only in S6/S7/S8")
					}

					route, err := routeForAnalyze(
						admission.Level,
						admission.LevelSource,
						admission.IntakeAssessment,
						admission.RouteSnapshot,
					)
					if err != nil {
						return err
					}
					eval := gate.EvaluateGPivot(gate.PivotKindReroute, true, admission.CurrentState, admission.Level)
					if eval.Status != model.GateStatusApproved {
						return fmt.Errorf("pivot blocked: %s", strings.Join(eval.Reasons, ", "))
					}

					admission.RouteSnapshot = route.RouteSnapshot
					if route.Level.IsValid() && route.Level != admission.Level {
						at := time.Now().UTC()
						admission.Level = route.Level
						admission.LevelSource = route.LevelSource
						admission.LastLevelUpdateAt = &at
						admission.LevelHistory = append(admission.LevelHistory, model.LevelHistoryEvent{
							Level:       route.Level,
							LevelSource: route.LevelSource,
							Reason:      "pivot",
							At:          at,
						})
					}

					if route.Level == model.LevelL2 || route.Level == model.LevelL3 {
						slug, err := pivotSlug(root, admission.IntakeAssessment.IntendedDelta)
						if err != nil {
							return err
						}
						if err := artifact.ScaffoldGovernedBundle(root, admission.RequestID, slug, route.Level); err != nil {
							return err
						}
						cfg, err := loadConfigAtRoot(root)
						if err != nil {
							return err
						}
						sealed, change, err := state.HandoffAdmissionToGoverned(
							admission,
							slug,
							route.Level,
							cfg.Execution.MaxLevelHistoryEntries,
						)
						if err != nil {
							return err
						}
						change.ActionHistory = append(change.ActionHistory, model.ActionEvent{
							Action:    "pivot",
							State:     change.CurrentState,
							Timestamp: time.Now().UTC(),
							Details: map[string]string{
								"kind": kind,
							},
						})
						if err := state.SaveAdmission(root, sealed); err != nil {
							return err
						}
						if err := state.SaveChange(root, change); err != nil {
							return err
						}
						view = pivotView{
							RequestID:    active.RequestID,
							Kind:         kind,
							LaneMode:     "governed",
							CurrentState: string(change.CurrentState),
							Level:        string(change.Level),
						}
					} else {
						admission.CurrentState = model.StateS6RunWaves
						admission.ActionHistory = append(admission.ActionHistory, model.ActionEvent{
							Action:    "pivot",
							State:     admission.CurrentState,
							Timestamp: time.Now().UTC(),
							Details: map[string]string{
								"kind": kind,
							},
						})
						if err := state.SaveAdmission(root, admission); err != nil {
							return err
						}
						view = pivotView{
							RequestID:    active.RequestID,
							Kind:         kind,
							LaneMode:     "admission_only",
							CurrentState: string(admission.CurrentState),
							Level:        string(admission.Level),
						}
					}
				case state.ActiveResolutionModeGoverned:
					change, err := state.LoadChange(root, active.RequestID)
					if err != nil {
						return err
					}
					if !isPivotState(change.CurrentState) {
						return fmt.Errorf("pivot is allowed only in S6/S7/S8")
					}
					if kind == string(gate.PivotKindRescope) && change.CurrentState != model.StateS6RunWaves {
						return fmt.Errorf("rescope requires governed S6_RUN_WAVES")
					}

					assessment := model.IntakeAssessment{
						IntentType:       "executable_change",
						IsExecutable:     true,
						Confidence:       0.9,
						ChangeTargets:    []string{"workspace"},
						IntendedDelta:    "reroute governed request",
						AcceptanceAnchor: "pivot applied",
					}
					if admission, err := state.LoadAdmission(root, active.RequestID); err == nil {
						assessment = admission.IntakeAssessment
					}
					route, err := routeForAnalyze(
						change.Level,
						change.LevelSource,
						assessment,
						change.RouteSnapshot,
					)
					if err != nil {
						return err
					}

					kindEnum := gate.PivotKind(kind)
					eval := gate.EvaluateGPivot(kindEnum, true, change.CurrentState, change.Level)
					if eval.Status != model.GateStatusApproved {
						return fmt.Errorf("pivot blocked: %s", strings.Join(eval.Reasons, ", "))
					}
					change.Gates[string(gate.GatePivot)] = model.GateRecord{
						GateID:    string(gate.GatePivot),
						Status:    eval.Status,
						Decision:  model.GateDecisionApprove,
						Reasons:   append([]string{}, eval.Reasons...),
						UpdatedAt: time.Now().UTC(),
					}

					change.RouteSnapshot = route.RouteSnapshot
					var nextState model.WorkflowState
					if kindEnum == gate.PivotKindRescope {
						next, err := action.ResolveLoopTransition(action.LoopTransitionInput{
							Level:             change.Level,
							CurrentState:      change.CurrentState,
							Trigger:           action.LoopTriggerPivotRescope,
							AnalyzeLevel:      route.Level,
							PivotGateApproved: true,
						})
						if err != nil {
							return err
						}
						nextState = next
					} else {
						switch route.Level {
						case model.LevelL3:
							nextState = model.StateS2Discover
						case model.LevelL2:
							nextState = model.StateS4SpecBundle
						default:
							nextState = model.StateS6RunWaves
						}
					}

					if route.Level.IsValid() && route.Level != change.Level {
						cfg, err := loadConfigAtRoot(root)
						if err != nil {
							return err
						}
						if err := state.ApplyLevelPivot(
							&change,
							route.Level,
							route.LevelSource,
							"pivot",
							time.Now().UTC(),
							cfg.Execution.MaxLevelHistoryEntries,
						); err != nil {
							return err
						}
					}
					change.CurrentState = nextState
					change.ActionHistory = append(change.ActionHistory, model.ActionEvent{
						Action:    "pivot",
						State:     change.CurrentState,
						Timestamp: time.Now().UTC(),
						Details: map[string]string{
							"kind": kind,
						},
					})
					if err := state.SaveChange(root, change); err != nil {
						return err
					}

					view = pivotView{
						RequestID:    active.RequestID,
						Kind:         kind,
						LaneMode:     "governed",
						CurrentState: string(change.CurrentState),
						Level:        string(change.Level),
					}
				default:
					return fmt.Errorf("unsupported active mode %q", active.Mode)
				}

				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			})
		},
	}
	cmd.Flags().StringVar(&opts.kind, "kind", "reroute", "Pivot kind: reroute|rescope")
	return cmd
}

func isPivotState(stateID model.WorkflowState) bool {
	return stateID == model.StateS6RunWaves || stateID == model.StateS7Review || stateID == model.StateS8Verify
}

func pivotSlug(root, intendedDelta string) (string, error) {
	title := strings.TrimSpace(intendedDelta)
	if title == "" {
		title = "pivot-change"
	}
	base := model.SlugifyTitle(title)
	return model.ResolveSlugCollision(base, func(candidate string) bool {
		_, err := os.Stat(filepath.Join(root, "aircraft", "changes", candidate))
		return err == nil
	}), nil
}
