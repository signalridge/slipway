package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	reviewengine "github.com/signalridge/speclane/internal/engine/review"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
)

type reviewOptions struct {
	changedOnly bool
	all         bool
	artifact    string
}

type reviewView struct {
	RequestID    string   `json:"request_id"`
	LaneMode     string   `json:"lane_mode"`
	CurrentState string   `json:"current_state"`
	Verdict      string   `json:"verdict"`
	Blockers     []string `json:"blockers,omitempty"`
}

func newReviewCmd() *cobra.Command {
	opts := reviewOptions{}
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Run review flow for current execution artifacts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("all") && cmd.Flags().Changed("changed-only") && opts.all && opts.changedOnly {
				return fmt.Errorf("`--changed-only` and `--all` are mutually exclusive")
			}
			reviewAll := opts.all || !opts.changedOnly

			if opts.artifact != "" {
				return fmt.Errorf(
					"`--artifact` is not supported in MVP; use default changed-only review or --all",
				)
			}

			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			return withWorkspaceStateLock(root, "review", func() error {
				active, err := ensureRequestScopedActive(root)
				if err != nil {
					return err
				}

				var view reviewView
				switch active.Mode {
				case state.ActiveResolutionModeAdmissionOnly:
					admission, err := state.LoadAdmission(root, active.RequestID)
					if err != nil {
						return err
					}
					if admission.AdmissionStatus != model.AdmissionStatusActive {
						return fmt.Errorf("review requires active request; current status=%s", admission.AdmissionStatus)
					}

					if err := ensureReviewEntryState(admission.CurrentState, admission.LatestFrozenRunSummaryVersion); err != nil {
						return err
					}
					if admission.CurrentState == model.StateS6RunWaves || admission.CurrentState == model.StateS8Verify {
						admission.CurrentState = model.StateS7Review
					}

					verdict, blockers := evaluateReviewVerdict(
						admission.TaskRuns,
						admission.LatestFrozenRunSummaryVersion,
						reviewAll,
					)
					if verdict == "fail" {
						admission.CurrentState = model.StateS6RunWaves
					}
					admission.ActionHistory = append(admission.ActionHistory, model.ActionEvent{
						Action:    "review",
						State:     admission.CurrentState,
						Timestamp: time.Now().UTC(),
						Details: map[string]string{
							"verdict": verdict,
						},
					})
					if err := state.SaveAdmission(root, admission); err != nil {
						return err
					}
					view = reviewView{
						RequestID:    active.RequestID,
						LaneMode:     "admission_only",
						CurrentState: string(admission.CurrentState),
						Verdict:      verdict,
						Blockers:     blockers,
					}
				case state.ActiveResolutionModeGoverned:
					change, err := state.LoadChange(root, active.RequestID)
					if err != nil {
						return err
					}
					if change.ChangeStatus != model.ChangeStatusActive {
						return fmt.Errorf("review requires active request; current status=%s", change.ChangeStatus)
					}

					if err := ensureReviewEntryState(change.CurrentState, change.LatestFrozenRunSummaryVersion); err != nil {
						return err
					}
					if change.CurrentState == model.StateS6RunWaves || change.CurrentState == model.StateS8Verify {
						change.CurrentState = model.StateS7Review
					}

					requiredSkillRecords, skillBlockers, err := evaluateRequiredSkills(
						root,
						change.RequestID,
						change.Level,
						model.StateS7Review,
						change.LatestFrozenRunSummaryVersion,
						false,
					)
					if err != nil {
						return err
					}

					verdict, blockers := evaluateReviewVerdict(
						change.TaskRuns,
						change.LatestFrozenRunSummaryVersion,
						reviewAll,
					)
					blockers = append(blockers, skillBlockers...)
					blockers = append(blockers, requiredLayerBlockers(change, requiredSkillRecords["artifact-review"], reviewAll)...)
					blockers = uniqueSorted(blockers)
					if len(blockers) > 0 {
						verdict = "fail"
					}

					intentDriftFailures := parseIntentDriftFailures(change.EvidenceRefs[reviewIntentDriftCounterKey])
					if verdict == "fail" && hasIntentDriftSignal(blockers, requiredSkillRecords["artifact-review"]) {
						intentDriftFailures++
					} else {
						intentDriftFailures = 0
					}
					change.EvidenceRefs[reviewIntentDriftCounterKey] = strconv.Itoa(intentDriftFailures)
					if intentDriftFailures >= 2 {
						blockers = uniqueSorted(append(blockers, "pivot_required:intent_drift"))
						verdict = "fail"
					}
					if verdict == "fail" {
						change.CurrentState = model.StateS6RunWaves
					}
					change.ActionHistory = append(change.ActionHistory, model.ActionEvent{
						Action:    "review",
						State:     change.CurrentState,
						Timestamp: time.Now().UTC(),
						Details: map[string]string{
							"verdict": verdict,
						},
					})
					if err := state.SaveChange(root, change); err != nil {
						return err
					}
					view = reviewView{
						RequestID:    active.RequestID,
						LaneMode:     "governed",
						CurrentState: string(change.CurrentState),
						Verdict:      verdict,
						Blockers:     blockers,
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

	cmd.Flags().BoolVar(&opts.changedOnly, "changed-only", true, "Review only changed/stale units")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Run full review")
	cmd.Flags().StringVar(&opts.artifact, "artifact", "", "Artifact path (unsupported in MVP)")
	return cmd
}

func ensureReviewEntryState(current model.WorkflowState, latestSummary int) error {
	switch current {
	case model.StateS7Review:
		return nil
	case model.StateS6RunWaves:
		if latestSummary < 1 {
			return fmt.Errorf("review from S6 requires frozen run summary; complete `spln do` first")
		}
		return nil
	case model.StateS8Verify:
		return nil
	default:
		return fmt.Errorf("review is allowed only in S6/S7/S8")
	}
}

func evaluateReviewVerdict(taskRuns map[string]model.TaskRun, latestSummary int, all bool) (string, []string) {
	if !all && latestSummary < 1 {
		return "fail", []string{"missing_frozen_run_summary"}
	}
	if all && len(taskRuns) == 0 {
		return "fail", []string{"no_task_runs_for_review"}
	}

	blockers := []string{}
	for key, run := range taskRuns {
		if !all && run.RunSummaryVersion != latestSummary {
			continue
		}
		if run.Verdict != model.TaskVerdictPass {
			blockers = append(blockers, "non_pass_task:"+run.TaskID)
		}
		if len(run.Blockers) > 0 {
			blockers = append(blockers, "task_blockers:"+key)
		}
	}
	if len(blockers) > 0 {
		return "fail", uniqueSorted(blockers)
	}
	return "pass", nil
}

const reviewIntentDriftCounterKey = "review.intent_drift_failures"

func requiredLayerBlockers(
	change model.ChangeState,
	artifactReviewEvidence model.EvidenceRecord,
	reviewAll bool,
) []string {
	if artifactReviewEvidence.SkillName == "" {
		return []string{"required_skill_missing:artifact-review"}
	}

	requiredLayers := map[reviewengine.Layer]struct{}{}
	requiredImplementation := reviewengine.RequiredImplementationLayers(change.Level, change.RouteSnapshot.GuardrailDomain)
	for _, layer := range requiredImplementation {
		requiredLayers[layer] = struct{}{}
	}

	artifactScope := artifactScopeForReview(change.Artifacts, reviewAll)
	for _, artifactID := range artifactScope {
		if artifactID == "explore" {
			continue
		}
		artifactName := artifactID + ".md"
		if artifactID == "change" {
			artifactName = "change.yaml"
		}
		for _, layer := range reviewengine.RequiredArtifactLayers(change.Level, change.RouteSnapshot.GuardrailDomain, artifactName) {
			requiredLayers[layer] = struct{}{}
		}
	}

	outcomes := parseLayerOutcomes(artifactReviewEvidence.References)
	blockers := []string{}
	for layer := range requiredLayers {
		passed, ok := outcomes[layer]
		if !ok {
			blockers = append(blockers, "review_layer_missing:"+string(layer))
			continue
		}
		if !passed {
			blockers = append(blockers, "review_layer_failed:"+string(layer))
		}
	}
	return uniqueSorted(blockers)
}

func artifactScopeForReview(artifacts map[string]model.ArtifactState, reviewAll bool) []string {
	keys := make([]string, 0, len(artifacts))
	for key, artifact := range artifacts {
		if reviewAll {
			keys = append(keys, key)
			continue
		}
		if artifact.State == model.ArtifactLifecycleDraft || artifact.State == model.ArtifactLifecycleStale {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 && !reviewAll {
		// If runtime state does not currently classify changed artifacts, keep lightweight manifest check.
		keys = append(keys, "change")
	}
	return uniqueSorted(keys)
}

func parseLayerOutcomes(references []string) map[reviewengine.Layer]bool {
	out := map[reviewengine.Layer]bool{}
	for _, ref := range references {
		raw := strings.TrimSpace(strings.ToLower(ref))
		if !strings.HasPrefix(raw, "layer:") {
			continue
		}
		raw = strings.TrimPrefix(raw, "layer:")
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			continue
		}
		layer := reviewengine.Layer(strings.ToUpper(strings.TrimSpace(parts[0])))
		switch strings.TrimSpace(parts[1]) {
		case "pass", "passed", "ok", "true":
			out[layer] = true
		case "fail", "failed", "false":
			out[layer] = false
		}
	}
	return out
}

func hasIntentDriftSignal(blockers []string, artifactReviewEvidence model.EvidenceRecord) bool {
	for _, blocker := range blockers {
		if strings.Contains(strings.ToLower(blocker), "intent_drift") {
			return true
		}
	}
	for _, ref := range artifactReviewEvidence.References {
		raw := strings.TrimSpace(strings.ToLower(ref))
		if raw == "intent_drift:true" || strings.HasPrefix(raw, "intent_drift:yes") {
			return true
		}
	}
	return false
}

func parseIntentDriftFailures(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return 0
	}
	return value
}
