package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/speclane/internal/engine/artifact"
	"github.com/signalridge/speclane/internal/engine/router"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
)

type newOptions struct {
	level          string
	assessmentFile string
}

func newNewCmd() *cobra.Command {
	opts := newOptions{}

	cmd := &cobra.Command{
		Use:   "new <description>",
		Short: "Create and route a new executable request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRootFromWD()
			if err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(root, ".spln")); errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("workspace is not initialized; run `spln init`")
			}
			return withWorkspaceStateLock(root, "new", func() error {
				records, err := state.DiscoverActiveRecords(root)
				if err != nil {
					return err
				}
				if len(records) > 0 {
					return fmt.Errorf("active request exists; finish or cancel it before creating a new one")
				}

				cfg, err := loadConfigAtRoot(root)
				if err != nil {
					return err
				}

				routeMode, fixedLevel, usedFallback, err := resolveNewLevelMode(
					opts.level,
					cfg,
					cmd.InOrStdin(),
					cmd.OutOrStdout(),
				)
				if err != nil {
					return err
				}

				description := strings.TrimSpace(args[0])
				assessment, scores, guardrailDomain, signals, usedFallbackAssessment, err := resolveRouteInputForNew(
					root,
					opts.assessmentFile,
					description,
				)
				if err != nil {
					return err
				}

				result, err := router.Route(router.RouteInput{
					Mode:             routeMode,
					FixedLevel:       fixedLevel,
					IntakeAssessment: assessment,
					Scores:           scores,
					GuardrailDomain:  guardrailDomain,
					Signals:          signals,
				})
				if err != nil {
					return err
				}
				if usedFallback {
					result.RouteSnapshot.RoutingRationale = append(result.RouteSnapshot.RoutingRationale, "config_invalid_level_mode_fallback:auto")
				}
				if usedFallbackAssessment {
					result.RouteSnapshot.RoutingRationale = append(result.RouteSnapshot.RoutingRationale, "routing_input:fallback_heuristics")
				}

				if result.Classification == router.ClassificationNonSpln {
					return newCLIError(
						categoryPrecondition,
						"non_spln_intent",
						"request is advisory/question and not executable",
						"Provide an executable change request, or use normal chat flow for advisory questions.",
						"",
						nil,
					)
				}
				if routeMode == router.LevelModeFixed && len(result.RouteSnapshot.BlockingConflicts) > 0 {
					return fmt.Errorf(
						"fixed level blocked by safety conflicts: %s; rerun with --level auto|L3",
						strings.Join(result.RouteSnapshot.BlockingConflicts, ", "),
					)
				}

				requestID, slug, err := createRequestIdentity(root, description, result.Level)
				if err != nil {
					return err
				}
				now := time.Now().UTC()

				admission := model.NewAdmissionState(requestID)
				admission.Level = result.Level
				admission.LevelSource = result.LevelSource
				admission.RouteSnapshot = result.RouteSnapshot
				admission.IntakeAssessment = result.IntakeAssessment
				admission.LevelHistory = []model.LevelHistoryEvent{
					{
						Level:       result.Level,
						LevelSource: result.LevelSource,
						Reason:      "new",
						At:          now,
					},
				}
				admission.LastLevelUpdateAt = &now

				if result.Level == model.LevelL1 {
					admission.CurrentState = model.StateS6RunWaves
					if err := state.SaveAdmission(root, admission); err != nil {
						return err
					}
					_, _ = fmt.Fprintf(
						cmd.OutOrStdout(),
						"created request %s level=%s state=%s\n",
						requestID,
						result.Level,
						admission.CurrentState,
					)
					return nil
				}

				if err := artifact.ScaffoldGovernedBundle(root, requestID, slug, result.Level); err != nil {
					return err
				}

				admission.CurrentState = model.StateS1Analyze
				sealed, change, err := state.HandoffAdmissionToGoverned(
					admission,
					slug,
					result.Level,
					cfg.Execution.MaxLevelHistoryEntries,
				)
				if err != nil {
					return err
				}
				if err := state.SaveAdmission(root, sealed); err != nil {
					return err
				}
				if err := state.SaveChange(root, change); err != nil {
					return err
				}

				_, _ = fmt.Fprintf(
					cmd.OutOrStdout(),
					"created request %s level=%s state=%s slug=%s\n",
					requestID,
					result.Level,
					change.CurrentState,
					slug,
				)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&opts.level, "level", "", "Routing mode/level: auto|L1|L2|L3")
	cmd.Flags().StringVar(&opts.assessmentFile, "assessment-file", "", "Structured routing input JSON file (LLM/skill output)")
	return cmd
}

func resolveNewLevelMode(
	levelFlag string,
	cfg model.Config,
	in io.Reader,
	out io.Writer,
) (router.LevelMode, model.Level, bool, error) {
	if levelFlag != "" {
		switch strings.ToUpper(strings.TrimSpace(levelFlag)) {
		case "AUTO":
			return router.LevelModeAuto, "", false, nil
		case "L1":
			return router.LevelModeFixed, model.LevelL1, false, nil
		case "L2":
			return router.LevelModeFixed, model.LevelL2, false, nil
		case "L3":
			return router.LevelModeFixed, model.LevelL3, false, nil
		default:
			return "", "", false, fmt.Errorf("invalid --level value %q; expected auto|L1|L2|L3", levelFlag)
		}
	}

	mode, fallback := cfg.EffectiveLevelMode()
	if isInteractiveTTY(in, out) {
		selected, err := promptLevelModeSelection(mode, in, out)
		if err != nil {
			return "", "", fallback, err
		}
		mode = selected
	}
	if mode == model.LevelModeAuto {
		return router.LevelModeAuto, "", fallback, nil
	}
	level := model.Level(mode)
	if !level.IsValid() {
		return router.LevelModeAuto, "", true, nil
	}
	return router.LevelModeFixed, level, fallback, nil
}

func promptLevelModeSelection(defaultMode model.LevelMode, in io.Reader, out io.Writer) (model.LevelMode, error) {
	if defaultMode == "" || !defaultMode.IsValid() {
		defaultMode = model.LevelModeAuto
	}

	if _, err := fmt.Fprintf(
		out,
		"Select level mode [auto/L1/L2/L3] (default: %s): ",
		string(defaultMode),
	); err != nil {
		return "", err
	}

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		return defaultMode, nil
	}
	switch strings.ToUpper(choice) {
	case "AUTO":
		return model.LevelModeAuto, nil
	case "L1":
		return model.LevelMode(model.LevelL1), nil
	case "L2":
		return model.LevelMode(model.LevelL2), nil
	case "L3":
		return model.LevelMode(model.LevelL3), nil
	default:
		return "", fmt.Errorf("invalid level mode %q; expected auto|L1|L2|L3", choice)
	}
}

func isInteractiveTTY(in io.Reader, out io.Writer) bool {
	inFile, ok := in.(*os.File)
	if !ok {
		return false
	}
	outFile, ok := out.(*os.File)
	if !ok {
		return false
	}

	inInfo, err := inFile.Stat()
	if err != nil {
		return false
	}
	outInfo, err := outFile.Stat()
	if err != nil {
		return false
	}
	return (inInfo.Mode()&os.ModeCharDevice) != 0 && (outInfo.Mode()&os.ModeCharDevice) != 0
}

func inferIntakeAssessment(description string) model.IntakeAssessment {
	text := strings.ToLower(strings.TrimSpace(description))
	assessment := model.IntakeAssessment{
		IntentType:       "executable_change",
		IsExecutable:     true,
		Confidence:       0.9,
		ChangeTargets:    []string{"workspace"},
		IntendedDelta:    description,
		AcceptanceAnchor: "code and tests updated",
		BlockingUnknowns: []string{},
		AuxiliarySignals: []string{},
	}
	if strings.HasSuffix(text, "?") || strings.Contains(text, "how do i") || strings.Contains(text, "what is") {
		assessment.IntentType = "question"
		assessment.IsExecutable = false
		assessment.ChangeTargets = nil
		assessment.Confidence = 0.9
	}
	hasUncertainty := strings.Contains(text, "not sure") || strings.Contains(text, "unclear")
	hasExecutableSignal := strings.Contains(text, "fix ") ||
		strings.Contains(text, "implement") ||
		strings.Contains(text, "refactor") ||
		strings.Contains(text, "重构") ||
		strings.Contains(text, "add ") ||
		strings.Contains(text, "新增") ||
		strings.Contains(text, "update ") ||
		strings.Contains(text, "更新") ||
		strings.Contains(text, "migrate") ||
		strings.Contains(text, "change ")

	if hasUncertainty {
		if hasExecutableSignal {
			assessment.BlockingUnknowns = []string{"clarify scope"}
			assessment.Confidence = 0.8
		} else {
			assessment.IntentType = "clarification_needed"
			assessment.Confidence = 0.55
		}
	}
	if strings.Contains(text, "new project") || strings.Contains(text, "greenfield") || strings.Contains(text, "从零开始") {
		assessment.AuxiliarySignals = append(assessment.AuxiliarySignals, "new_project")
	}
	if strings.Contains(text, "major refactor") ||
		strings.Contains(text, "re-architect") ||
		strings.Contains(text, "跨模块") ||
		strings.Contains(text, "跨服务") {
		assessment.AuxiliarySignals = append(assessment.AuxiliarySignals, "major_refactor")
	}
	return assessment
}

func inferScores(description string) model.Scores {
	text := strings.ToLower(description)
	s := model.Scores{
		Novelty:           1,
		Ambiguity:         1,
		Impact:            1,
		Risk:              1,
		ReversibilityCost: 1,
	}
	if strings.Contains(text, "refactor") || strings.Contains(text, "re-architect") {
		s.Novelty = 2
		s.Ambiguity = 3
		s.Impact = 3
	}
	if strings.Contains(text, "migration") || strings.Contains(text, "schema") {
		s.Risk = 3
		s.ReversibilityCost = 3
	}
	return s
}

func inferGuardrailDomain(description string) string {
	text := strings.ToLower(description)
	switch {
	case strings.Contains(text, "auth"), strings.Contains(text, "oauth"), strings.Contains(text, "rbac"):
		return "auth_authz"
	case strings.Contains(text, "credential"), strings.Contains(text, "secret"), strings.Contains(text, "token"):
		return "security_credentials"
	case strings.Contains(text, "pii"), strings.Contains(text, "privacy"):
		return "privacy_pii"
	case strings.Contains(text, "payment"), strings.Contains(text, "billing"), strings.Contains(text, "financial"):
		return "financial_flows"
	case strings.Contains(text, "schema"), strings.Contains(text, "migration"):
		return "schema_data_migration"
	case strings.Contains(text, "delete"), strings.Contains(text, "irreversible"):
		return "irreversible_operations"
	case strings.Contains(text, "api contract"), strings.Contains(text, "public api"):
		return "external_api_contracts"
	default:
		return ""
	}
}

func createRequestIdentity(root, title string, level model.Level) (requestID, slug string, err error) {
	if level == model.LevelL2 || level == model.LevelL3 {
		return router.GenerateRequestV1(title, func(candidate string) bool {
			_, statErr := os.Stat(filepath.Join(root, "aircraft", "changes", candidate))
			return statErr == nil
		})
	}
	requestID, err = model.NewRequestID()
	return requestID, "", err
}

type routeInputEnvelope struct {
	IntakeAssessment model.IntakeAssessment `json:"intake_assessment"`
	Scores           model.Scores           `json:"scores"`
	GuardrailDomain  string                 `json:"guardrail_domain"`
	Signals          router.RouteSignals    `json:"signals"`
}

func loadRouteInputEnvelope(path string) (model.IntakeAssessment, model.Scores, string, router.RouteSignals, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.IntakeAssessment{}, model.Scores{}, "", router.RouteSignals{}, err
	}

	payload := routeInputEnvelope{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return model.IntakeAssessment{}, model.Scores{}, "", router.RouteSignals{}, err
	}

	if payload.IntakeAssessment.IntentType == "" {
		return model.IntakeAssessment{}, model.Scores{}, "", router.RouteSignals{}, fmt.Errorf(
			"assessment-file missing intake_assessment.intent_type",
		)
	}
	if err := payload.Scores.Validate(); err != nil {
		return model.IntakeAssessment{}, model.Scores{}, "", router.RouteSignals{}, fmt.Errorf(
			"assessment-file scores invalid: %w",
			err,
		)
	}
	if !payload.Signals.NewProject && !payload.Signals.MajorRefactor && len(payload.Signals.Rationale) == 0 {
		payload.Signals = router.DeriveRouteSignals(
			payload.IntakeAssessment,
			router.WorkspaceSignals{HasInScopeSourceFiles: true},
		)
	}

	return payload.IntakeAssessment, payload.Scores, payload.GuardrailDomain, payload.Signals, nil
}

func resolveRouteInputForNew(
	root string,
	assessmentFile string,
	description string,
) (model.IntakeAssessment, model.Scores, string, router.RouteSignals, bool, error) {
	if strings.TrimSpace(assessmentFile) != "" {
		assessment, scores, guardrailDomain, signals, err := loadRouteInputEnvelope(assessmentFile)
		return assessment, scores, guardrailDomain, signals, false, err
	}

	assessment, scores, guardrailDomain, signals, loadedFromEvidence, err := loadLatestRouteInputFromIntakeEvidence(root)
	if err != nil {
		return model.IntakeAssessment{}, model.Scores{}, "", router.RouteSignals{}, false, err
	}
	if loadedFromEvidence {
		return assessment, scores, guardrailDomain, signals, false, nil
	}

	if !heuristicFallbackEnabled() {
		return model.IntakeAssessment{}, model.Scores{}, "", router.RouteSignals{}, false, newCLIError(
			categoryPrecondition,
			"assessment_input_required",
			"LLM intake assessment is required for routing when --assessment-file is omitted",
			"Provide --assessment-file from intake-analysis skill output, or set SPLN_ALLOW_HEURISTIC_FALLBACK=1 for legacy local heuristics.",
			"",
			nil,
		)
	}

	assessment = inferIntakeAssessment(description)
	scores = inferScores(description)
	guardrailDomain = inferGuardrailDomain(description)
	signals = router.DeriveRouteSignals(assessment, router.WorkspaceSignals{HasInScopeSourceFiles: true})
	return assessment, scores, guardrailDomain, signals, true, nil
}

func loadLatestRouteInputFromIntakeEvidence(
	root string,
) (model.IntakeAssessment, model.Scores, string, router.RouteSignals, bool, error) {
	records, _, err := loadSkillEvidenceFiles(root, "")
	if err != nil {
		return model.IntakeAssessment{}, model.Scores{}, "", router.RouteSignals{}, false, err
	}
	if len(records) == 0 {
		return model.IntakeAssessment{}, model.Scores{}, "", router.RouteSignals{}, false, nil
	}

	slices.SortFunc(records, func(a, b skillEvidenceFile) int {
		at := normalizedEvidenceTimestamp(a.Record)
		bt := normalizedEvidenceTimestamp(b.Record)
		if at.After(bt) {
			return -1
		}
		if bt.After(at) {
			return 1
		}
		if a.Path < b.Path {
			return -1
		}
		if a.Path > b.Path {
			return 1
		}
		return 0
	})

	for _, candidate := range records {
		record := candidate.Record
		if record.SkillName != "intake-analysis" {
			continue
		}
		if record.Verdict != model.EvidenceVerdictPass || len(record.Blockers) > 0 {
			continue
		}

		reference, ok := extractRouteInputReference(root, record.References)
		if !ok {
			continue
		}
		assessment, scores, guardrailDomain, signals, err := loadRouteInputEnvelope(reference)
		if err != nil {
			return model.IntakeAssessment{}, model.Scores{}, "", router.RouteSignals{}, false, err
		}
		return assessment, scores, guardrailDomain, signals, true, nil
	}

	return model.IntakeAssessment{}, model.Scores{}, "", router.RouteSignals{}, false, nil
}

func extractRouteInputReference(root string, references []string) (string, bool) {
	for _, reference := range references {
		ref := strings.TrimSpace(reference)
		if ref == "" {
			continue
		}
		switch {
		case strings.HasPrefix(ref, "route_input:"):
			path := strings.TrimSpace(strings.TrimPrefix(ref, "route_input:"))
			if path == "" {
				continue
			}
			if filepath.IsAbs(path) {
				return path, true
			}
			return filepath.Join(root, path), true
		case strings.HasPrefix(ref, "assessment_file:"):
			path := strings.TrimSpace(strings.TrimPrefix(ref, "assessment_file:"))
			if path == "" {
				continue
			}
			if filepath.IsAbs(path) {
				return path, true
			}
			return filepath.Join(root, path), true
		}
	}
	return "", false
}
