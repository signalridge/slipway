package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/signalridge/speclane/internal/engine/action"
	"github.com/signalridge/speclane/internal/engine/router"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
)

type analyzeView struct {
	RequestID string   `json:"request_id"`
	LaneMode  string   `json:"lane_mode"`
	State     string   `json:"current_state"`
	Level     string   `json:"level"`
	Blockers  []string `json:"blockers,omitempty"`
}

func newAnalyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Refresh intake analysis for the active request",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			return withWorkspaceStateLock(root, "analyze", func() error {
				active, err := ensureRequestScopedActive(root)
				if err != nil {
					return err
				}

				var view analyzeView
				switch active.Mode {
				case state.ActiveResolutionModeAdmissionOnly:
					admission, err := state.LoadAdmission(root, active.RequestID)
					if err != nil {
						return err
					}
					if admission.AdmissionStatus != model.AdmissionStatusActive {
						return fmt.Errorf("analyze requires active request; current status=%s", admission.AdmissionStatus)
					}
					if admission.CurrentState == model.StateDone {
						return fmt.Errorf("analyze is not allowed for DONE requests; start a new request")
					}

					assessment := admission.IntakeAssessment
					if strings.TrimSpace(assessment.IntendedDelta) == "" {
						assessment.IntendedDelta = "refresh active request analysis"
					}
					route, err := routeForAnalyze(
						admission.Level,
						admission.LevelSource,
						assessment,
						admission.RouteSnapshot,
					)
					if err != nil {
						return err
					}
					if route.LevelSource == model.LevelSourceUserSelected {
						conflicts := action.EvaluateFixedLevelSafety(admission.Level, route.RouteSnapshot.GuardrailDomain)
						route.RouteSnapshot.BlockingConflicts = conflicts
					}

					if err := action.ApplyAnalyzeOverrideAdmission(&admission, route.RouteSnapshot.BlockingConflicts); err != nil {
						return err
					}
					admission.RouteSnapshot = route.RouteSnapshot
					admission.ActionHistory = append(admission.ActionHistory, model.ActionEvent{
						Action:    "analyze",
						State:     admission.CurrentState,
						Timestamp: time.Now().UTC(),
					})
					if err := state.SaveAdmission(root, admission); err != nil {
						return err
					}
					view = analyzeView{
						RequestID: active.RequestID,
						LaneMode:  "admission_only",
						State:     string(admission.CurrentState),
						Level:     string(admission.Level),
						Blockers:  append([]string{}, admission.RouteSnapshot.BlockingConflicts...),
					}
				case state.ActiveResolutionModeGoverned:
					change, err := state.LoadChange(root, active.RequestID)
					if err != nil {
						return err
					}
					if change.ChangeStatus != model.ChangeStatusActive {
						return fmt.Errorf("analyze requires active request; current status=%s", change.ChangeStatus)
					}
					if change.CurrentState == model.StateDone {
						return fmt.Errorf("analyze is not allowed for DONE requests; start a new request")
					}

					assessment := model.IntakeAssessment{
						IntentType:       "executable_change",
						IsExecutable:     true,
						Confidence:       0.9,
						ChangeTargets:    []string{"workspace"},
						IntendedDelta:    "refresh governed request analysis",
						AcceptanceAnchor: "route metadata refreshed",
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
					if route.LevelSource == model.LevelSourceUserSelected {
						conflicts := action.EvaluateFixedLevelSafety(change.Level, route.RouteSnapshot.GuardrailDomain)
						route.RouteSnapshot.BlockingConflicts = conflicts
					}

					if err := action.ApplyAnalyzeOverrideChange(&change, route.RouteSnapshot.BlockingConflicts); err != nil {
						return err
					}
					change.RouteSnapshot = route.RouteSnapshot
					change.ActionHistory = append(change.ActionHistory, model.ActionEvent{
						Action:    "analyze",
						State:     change.CurrentState,
						Timestamp: time.Now().UTC(),
					})
					if err := state.SaveChange(root, change); err != nil {
						return err
					}
					view = analyzeView{
						RequestID: active.RequestID,
						LaneMode:  "governed",
						State:     string(change.CurrentState),
						Level:     string(change.Level),
						Blockers:  append([]string{}, change.RouteSnapshot.BlockingConflicts...),
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
	return cmd
}

func routeForAnalyze(
	level model.Level,
	levelSource model.LevelSource,
	assessment model.IntakeAssessment,
	existingSnapshot model.RouteSnapshot,
) (router.RouteResult, error) {
	mode := router.LevelModeAuto
	fixed := model.Level("")
	if levelSource == model.LevelSourceUserSelected && level.IsValid() {
		mode = router.LevelModeFixed
		fixed = level
	}

	scores := existingSnapshot.Scores
	guardrailDomain := existingSnapshot.GuardrailDomain
	signals := router.DeriveRouteSignals(assessment, router.WorkspaceSignals{HasInScopeSourceFiles: true})
	if scores == (model.Scores{}) {
		if !heuristicFallbackEnabled() {
			return router.RouteResult{}, fmt.Errorf(
				"analyze requires route_snapshot.scores; provide LLM routing inputs or set %s=1 for legacy heuristics",
				heuristicFallbackEnv,
			)
		}
		scores = inferScores(assessment.IntendedDelta)
		signals.Rationale = append(signals.Rationale, "routing_input:fallback_heuristics")
	}
	if strings.TrimSpace(guardrailDomain) == "" && heuristicFallbackEnabled() {
		guardrailDomain = inferGuardrailDomain(assessment.IntendedDelta)
	}

	return router.Route(router.RouteInput{
		Mode:             mode,
		FixedLevel:       fixed,
		IntakeAssessment: assessment,
		Scores:           scores,
		GuardrailDomain:  guardrailDomain,
		Signals:          signals,
	})
}
