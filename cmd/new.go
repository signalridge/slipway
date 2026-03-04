package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/signalridge/speclane/internal/engine/artifact"
	"github.com/signalridge/speclane/internal/engine/router"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type newOptions struct {
	level string
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

				routeMode, fixedLevel, usedFallback, err := resolveNewLevelMode(opts.level, cfg)
				if err != nil {
					return err
				}

				description := strings.TrimSpace(args[0])
				assessment := inferIntakeAssessment(description)
				result, err := router.Route(router.RouteInput{
					Mode:             routeMode,
					FixedLevel:       fixedLevel,
					IntakeAssessment: assessment,
					Scores:           inferScores(description),
					GuardrailDomain:  inferGuardrailDomain(description),
					Signals:          router.DeriveRouteSignals(assessment, router.WorkspaceSignals{HasInScopeSourceFiles: true}),
				})
				if err != nil {
					return err
				}
				if usedFallback {
					result.RouteSnapshot.RoutingRationale = append(result.RouteSnapshot.RoutingRationale, "config_invalid_level_mode_fallback:auto")
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
					fmt.Fprintf(cmd.OutOrStdout(), "created request %s level=%s state=%s\n", requestID, result.Level, admission.CurrentState)
					return nil
				}

				if err := artifact.ScaffoldGovernedBundle(root, slug, result.Level); err != nil {
					return err
				}
				if err := writeChangeManifest(root, requestID, slug, result.Level); err != nil {
					return err
				}

				admission.CurrentState = model.StateS1Analyze
				sealed, change, err := state.HandoffAdmissionToGoverned(admission, slug, result.Level)
				if err != nil {
					return err
				}
				if err := state.SaveAdmission(root, sealed); err != nil {
					return err
				}
				if err := state.SaveChange(root, change); err != nil {
					return err
				}

				fmt.Fprintf(cmd.OutOrStdout(), "created request %s level=%s state=%s slug=%s\n", requestID, result.Level, change.CurrentState, slug)
				return nil
			})
		},
	}

	cmd.Flags().StringVar(&opts.level, "level", "", "Routing mode/level: auto|L1|L2|L3")
	return cmd
}

func resolveNewLevelMode(levelFlag string, cfg model.Config) (router.LevelMode, model.Level, bool, error) {
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
	if mode == model.LevelModeAuto {
		return router.LevelModeAuto, "", fallback, nil
	}
	level := model.Level(mode)
	if !level.IsValid() {
		return router.LevelModeAuto, "", true, nil
	}
	return router.LevelModeFixed, level, fallback, nil
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
		strings.Contains(text, "add ") ||
		strings.Contains(text, "update ") ||
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

func writeChangeManifest(root, requestID, slug string, level model.Level) error {
	manifest := state.ChangeManifest{
		RequestID:      requestID,
		Slug:           slug,
		CreatedAtLevel: level,
	}
	b, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	path := filepath.Join(root, "aircraft", "changes", slug, "change.yaml")
	return os.WriteFile(path, b, 0o644)
}
