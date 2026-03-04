package router

import (
	"fmt"
	"slices"
	"strings"

	"github.com/signalridge/speclane/internal/model"
)

type Classification string

const (
	ClassificationExecutable Classification = "executable"
	ClassificationNonSpln    Classification = "non_spln"
)

type LevelMode string

const (
	LevelModeAuto  LevelMode = "auto"
	LevelModeFixed LevelMode = "fixed"
)

type WorkspaceSignals struct {
	HasInScopeSourceFiles bool
	ScopeTouchCount       int
	WholeSystemScope      bool
}

type RouteSignals struct {
	NewProject    bool
	MajorRefactor bool
	Rationale     []string
}

type RouteInput struct {
	Mode             LevelMode
	FixedLevel       model.Level
	IntakeAssessment model.IntakeAssessment
	Scores           model.Scores
	GuardrailDomain  string
	Signals          RouteSignals
}

type RouteResult struct {
	Classification   Classification         `json:"classification"`
	Level            model.Level            `json:"level,omitempty"`
	LevelSource      model.LevelSource      `json:"level_source,omitempty"`
	IntakeAssessment model.IntakeAssessment `json:"intake_assessment"`
	RouteSnapshot    model.RouteSnapshot    `json:"route_snapshot"`
}

func CanonicalizeGuardrailDomain(input string) string {
	raw := strings.ToLower(strings.TrimSpace(input))
	if raw == "" {
		return ""
	}

	switch raw {
	case "auth/authz", "auth_authz":
		return "auth_authz"
	case "security/credentials", "security_credentials":
		return "security_credentials"
	case "privacy/pii", "privacy_pii":
		return "privacy_pii"
	case "financial flows", "financial_flows":
		return "financial_flows"
	case "schema/data migration", "schema_data_migration":
		return "schema_data_migration"
	case "irreversible operations", "irreversible_operations":
		return "irreversible_operations"
	case "external api contracts", "external_api_contracts":
		return "external_api_contracts"
	default:
		return strings.ReplaceAll(strings.ReplaceAll(raw, "/", "_"), " ", "_")
	}
}

func ClassifyIntake(assessment model.IntakeAssessment) (Classification, []string) {
	if (assessment.IntentType == "advisory" || assessment.IntentType == "question") &&
		!assessment.IsExecutable &&
		assessment.Confidence >= 0.75 {
		return ClassificationNonSpln, nil
	}

	if assessment.IsExecutable &&
		assessment.Confidence >= 0.65 &&
		(len(assessment.ChangeTargets) > 0 || strings.TrimSpace(assessment.IntendedDelta) != "") {
		rationale := []string{}
		if len(assessment.BlockingUnknowns) > 0 {
			rationale = append(rationale, "execution_unknowns_present")
		}
		return ClassificationExecutable, rationale
	}

	return ClassificationNonSpln, []string{"non_spln:clarification_required"}
}

func DeriveRouteSignals(assessment model.IntakeAssessment, ws WorkspaceSignals) RouteSignals {
	s := RouteSignals{Rationale: []string{}}
	aux := make([]string, 0, len(assessment.AuxiliarySignals))
	for _, signal := range assessment.AuxiliarySignals {
		signal = strings.ToLower(strings.TrimSpace(signal))
		if signal == "" {
			continue
		}
		aux = append(aux, signal)
	}

	hasNewProjectSignal := slices.Contains(aux, "new_project")
	hasMajorRefactorSignal := slices.Contains(aux, "major_refactor")
	multiScope := len(assessment.ChangeTargets) >= 2 || ws.ScopeTouchCount >= 2 || ws.WholeSystemScope

	if assessment.Confidence >= 0.65 && assessment.IsExecutable {
		if hasNewProjectSignal || (!ws.HasInScopeSourceFiles && len(assessment.ChangeTargets) > 0) {
			s.NewProject = true
		}
		if hasMajorRefactorSignal && multiScope {
			s.MajorRefactor = true
		}
	}

	if hasNewProjectSignal && assessment.Confidence < 0.65 {
		s.Rationale = append(s.Rationale, "signal_uncertain:new_project")
	}
	if hasMajorRefactorSignal && assessment.Confidence < 0.65 {
		s.Rationale = append(s.Rationale, "signal_uncertain:major_refactor")
	}

	return s
}

func Route(input RouteInput) (RouteResult, error) {
	if err := input.Scores.Validate(); err != nil {
		return RouteResult{}, err
	}

	classification, rationale := ClassifyIntake(input.IntakeAssessment)
	guardrailDomain := CanonicalizeGuardrailDomain(input.GuardrailDomain)
	effectiveScores := input.Scores
	if guardrailDomain != "" && effectiveScores.Risk < 3 {
		effectiveScores.Risk = 3
	}

	snapshot := model.RouteSnapshot{
		Scores:           input.Scores,
		GuardrailDomain:  guardrailDomain,
		RoutingRationale: append([]string{}, rationale...),
	}
	snapshot.RoutingRationale = append(snapshot.RoutingRationale, input.Signals.Rationale...)

	result := RouteResult{
		Classification:   classification,
		IntakeAssessment: input.IntakeAssessment,
		RouteSnapshot:    snapshot,
	}

	if classification == ClassificationNonSpln {
		return result, nil
	}

	switch input.Mode {
	case LevelModeAuto:
		level := autoLevel(effectiveScores, guardrailDomain, input.IntakeAssessment, input.Signals, &snapshot)
		result.Level = level
		result.LevelSource = model.LevelSourceAuto
	case LevelModeFixed:
		if !input.FixedLevel.IsValid() {
			return RouteResult{}, fmt.Errorf("invalid fixed level: %q", input.FixedLevel)
		}
		result.Level = input.FixedLevel
		result.LevelSource = model.LevelSourceUserSelected
		if guardrailDomain != "" && input.FixedLevel != model.LevelL3 {
			result.RouteSnapshot.BlockingConflicts = append(result.RouteSnapshot.BlockingConflicts, "fixed_level_guardrail_conflict")
		}
	default:
		return RouteResult{}, fmt.Errorf("invalid mode %q", input.Mode)
	}

	return result, nil
}

func autoLevel(
	scores model.Scores,
	guardrailDomain string,
	assessment model.IntakeAssessment,
	signals RouteSignals,
	snapshot *model.RouteSnapshot,
) model.Level {
	controlScore := scores.ControlScore()

	if len(assessment.BlockingUnknowns) > 0 {
		snapshot.RoutingRationale = append(snapshot.RoutingRationale, "auto_route:blocking_unknowns_to_l3")
		return model.LevelL3
	}
	if signals.NewProject || signals.MajorRefactor {
		snapshot.RoutingRationale = append(snapshot.RoutingRationale, "auto_route:high_discovery_signal")
		return model.LevelL3
	}
	if scores.Ambiguity >= 3 && controlScore >= 8 {
		snapshot.RoutingRationale = append(snapshot.RoutingRationale, "auto_route:high_ambiguity_control_pressure")
		return model.LevelL3
	}
	if guardrailDomain != "" {
		snapshot.RoutingRationale = append(snapshot.RoutingRationale, "auto_route:guardrail_floor")
		return model.LevelL3
	}
	if controlScore >= 8 {
		snapshot.RoutingRationale = append(snapshot.RoutingRationale, "auto_route:high_control")
		return model.LevelL2
	}
	snapshot.RoutingRationale = append(snapshot.RoutingRationale, "auto_route:default_l1")
	return model.LevelL1
}

func GenerateRequestV1(title string, slugExists func(string) bool) (requestID string, slug string, err error) {
	requestID, err = model.NewRequestID()
	if err != nil {
		return "", "", err
	}

	baseSlug := model.SlugifyTitle(title)
	if slugExists == nil {
		return requestID, baseSlug, nil
	}
	return requestID, model.ResolveSlugCollision(baseSlug, slugExists), nil
}
