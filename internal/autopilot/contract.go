// Package autopilot defines Slipway's versioned host protocol and pure routing rules.
package autopilot

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

const ContractVersion = 1

const (
	maxOutcomeBytes              = 1 << 20
	maxActionContextBytes        = 128 << 10
	maxActionBriefBytes          = 8 << 10
	maxSuggestedActionBriefBytes = 8 << 10
	maxActionBytes               = 256 << 10
	DestructiveScopeVersion      = 1
)

type ActionKind string

const (
	ActionOrient    ActionKind = "orient"
	ActionClarify   ActionKind = "clarify"
	ActionImplement ActionKind = "implement"
	ActionReview    ActionKind = "review"
	ActionSummarize ActionKind = "summarize"
)

type OutcomeStatus string

const (
	OutcomeCompleted  OutcomeStatus = "completed"
	OutcomeNeedsInput OutcomeStatus = "needs_input"
	OutcomePartial    OutcomeStatus = "partial"
	OutcomeError      OutcomeStatus = "error"
)

type ImplementationResult string

const (
	ImplementationApplied   ImplementationResult = "applied"
	ImplementationPartial   ImplementationResult = "partial"
	ImplementationNotNeeded ImplementationResult = "not_needed"
	ImplementationUnable    ImplementationResult = "unable"
)

type ReviewResult string

const (
	ReviewNoFindings   ReviewResult = "no_findings_reported"
	ReviewFindings     ReviewResult = "findings_reported"
	ReviewInconclusive ReviewResult = "inconclusive"
	ReviewNotRun       ReviewResult = "not_run"
	ReviewError        ReviewResult = "error"
)

type RunState string

const (
	RunActive  RunState = "active"
	RunPaused  RunState = "paused"
	RunEnded   RunState = "ended"
	RunStopped RunState = "stopped"
)

type PauseReason string

const (
	PauseDecisionRequired       PauseReason = "decision_required"
	PauseDestructiveConfirm     PauseReason = "destructive_confirmation_required"
	PauseEnvironmentUnavailable PauseReason = "environment_unavailable"
	PauseBudgetExhausted        PauseReason = "budget_exhausted"
)

type ActionSourceKind string

const ActionSourceChangeIssue ActionSourceKind = "change_issue"

type DestructiveTargetKind string

const (
	DestructiveTargetPath             DestructiveTargetKind = "path"
	DestructiveTargetGitRef           DestructiveTargetKind = "git_ref"
	DestructiveTargetExternalResource DestructiveTargetKind = "external_resource"
	DestructiveTargetDataDomain       DestructiveTargetKind = "data_domain"
)

type ActionSource struct {
	Kind                 ActionSourceKind `json:"kind"`
	CanonicalURL         string           `json:"canonical_url"`
	IssueID              string           `json:"issue_id"`
	SourceRevision       string           `json:"source_revision"`
	RequirementsRevision string           `json:"requirements_revision"`
}

type DestructiveTarget struct {
	Kind  DestructiveTargetKind `json:"kind"`
	Value string                `json:"value"`
}

type DestructiveRequest struct {
	RequestID   string              `json:"request_id"`
	Targets     []DestructiveTarget `json:"targets"`
	Impact      string              `json:"impact"`
	ScopeSHA256 string              `json:"scope_sha256"`
}

type DestructiveAuthorization struct {
	RequestID           string              `json:"request_id"`
	OriginatingActionID string              `json:"originating_action_id"`
	ScopeVersion        int                 `json:"scope_version"`
	ScopeSHA256         string              `json:"scope_sha256"`
	Targets             []DestructiveTarget `json:"targets"`
	Impact              string              `json:"impact"`
	ConfirmedAt         string              `json:"confirmed_at"`
}

// Action is a bounded unit of host work. RemainingBudget is the number of
// further Actions that may be issued after this one.
type Action struct {
	ContractVersion          int                       `json:"contract_version"`
	RunID                    string                    `json:"run_id"`
	ActionID                 string                    `json:"action_id"`
	Kind                     ActionKind                `json:"kind"`
	Goal                     string                    `json:"goal"`
	Brief                    string                    `json:"brief"`
	Context                  string                    `json:"context"`
	Source                   *ActionSource             `json:"source,omitempty"`
	Requirements             *AcceptedRequirements     `json:"requirements,omitempty"`
	DestructiveAuthorization *DestructiveAuthorization `json:"destructive_authorization,omitempty"`
	RemainingBudget          int                       `json:"remaining_budget"`
}

type Activity struct {
	Kind     string `json:"kind"`
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Summary  string `json:"summary"`
}

type SuggestedAction struct {
	Kind  ActionKind `json:"kind"`
	Brief string     `json:"brief"`
}

type Pause struct {
	Reason             PauseReason         `json:"reason"`
	Question           string              `json:"question"`
	DestructiveRequest *DestructiveRequest `json:"destructive_request"`
}

type Implementation struct {
	Result        ImplementationResult `json:"result"`
	FilesChanged  []string             `json:"files_changed"`
	Activities    []Activity           `json:"activities"`
	Uncertainties []string             `json:"uncertainties"`
	Attempts      int                  `json:"attempts"`
}

type Finding struct {
	Location string `json:"location"`
	Summary  string `json:"summary"`
	Detail   string `json:"detail"`
}

type Review struct {
	Result        ReviewResult `json:"result"`
	Findings      []Finding    `json:"findings"`
	Uncertainties []string     `json:"uncertainties"`
}

// Outcome reports what the host observed and did. Every public JSON field is
// mandatory; inapplicable object branches are encoded as null.
type Outcome struct {
	ContractVersion  int               `json:"contract_version"`
	ActionID         string            `json:"action_id"`
	Status           OutcomeStatus     `json:"status"`
	Summary          string            `json:"summary"`
	Observations     []string          `json:"observations"`
	KnownIssues      []string          `json:"known_issues"`
	SuggestedActions []SuggestedAction `json:"suggested_actions"`
	Pause            *Pause            `json:"pause"`
	Implementation   *Implementation   `json:"implementation"`
	Review           *Review           `json:"review"`
	// RawSHA256 is set by DecodeOutcome from the exact input bytes and is never serialized.
	RawSHA256 string `json:"-"`
}

type VersionError struct {
	Received int
}

func (err *VersionError) Error() string {
	return fmt.Sprintf("contract_version %d is unsupported; supported version is %d", err.Received, ContractVersion)
}

func ValidActionKind(kind ActionKind) bool {
	switch kind {
	case ActionOrient, ActionClarify, ActionImplement, ActionReview, ActionSummarize:
		return true
	default:
		return false
	}
}

func (action Action) Validate() error {
	if action.ContractVersion != ContractVersion {
		return &VersionError{Received: action.ContractVersion}
	}
	for name, value := range map[string]string{
		"run_id": action.RunID, "action_id": action.ActionID, "goal": action.Goal,
		"brief": action.Brief, "context": action.Context,
	} {
		if err := requireNonEmptyUTF8(name, value); err != nil {
			return err
		}
	}
	if !utf8.ValidString(string(action.Kind)) || !ValidActionKind(action.Kind) {
		return fmt.Errorf("invalid action kind %q", action.Kind)
	}
	if len(action.Brief) > maxActionBriefBytes {
		return fmt.Errorf("brief exceeds %d bytes", maxActionBriefBytes)
	}
	if len(action.Context) > maxActionContextBytes {
		return fmt.Errorf("context exceeds %d bytes", maxActionContextBytes)
	}
	if action.RemainingBudget < 0 {
		return errors.New("remaining_budget cannot be negative")
	}
	if (action.Source == nil) != (action.Requirements == nil) {
		return errors.New("source and requirements must either both be present or both be omitted")
	}
	if action.Source != nil {
		if err := validateActionSource(*action.Source); err != nil {
			return err
		}
		if err := validateAcceptedRequirements(*action.Requirements); err != nil {
			return err
		}
	}
	if action.DestructiveAuthorization != nil {
		if action.Kind != ActionImplement {
			return errors.New("destructive_authorization is only valid for implement actions")
		}
		if err := validateDestructiveAuthorization(*action.DestructiveAuthorization); err != nil {
			return err
		}
	}
	encoded, err := json.Marshal(action)
	if err != nil {
		return fmt.Errorf("encode action: %w", err)
	}
	if len(encoded) > maxActionBytes {
		return fmt.Errorf("encoded action exceeds %d bytes", maxActionBytes)
	}
	return nil
}

func validateActionSource(source ActionSource) error {
	if source.Kind != ActionSourceChangeIssue {
		return fmt.Errorf("source kind %q is unsupported", source.Kind)
	}
	for name, value := range map[string]string{
		"source.canonical_url": source.CanonicalURL,
		"source.issue_id":      source.IssueID,
	} {
		if err := requireNonEmptyUTF8(name, value); err != nil {
			return err
		}
	}
	if !validSHA256(source.SourceRevision) {
		return errors.New("source.source_revision must use lowercase sha256:<64 hex> format")
	}
	if !validSHA256(source.RequirementsRevision) {
		return errors.New("source.requirements_revision must use lowercase sha256:<64 hex> format")
	}
	return nil
}

func validateAcceptedRequirements(requirements AcceptedRequirements) error {
	for name, value := range map[string]string{
		"requirements.outcome_markdown":             requirements.OutcomeMarkdown,
		"requirements.requirements_markdown":        requirements.RequirementsMarkdown,
		"requirements.acceptance_examples_markdown": requirements.AcceptanceExamplesMarkdown,
		"requirements.constraints_markdown":         requirements.ConstraintsMarkdown,
		"requirements.non_goals_markdown":           requirements.NonGoalsMarkdown,
	} {
		if !utf8.ValidString(value) {
			return fmt.Errorf("%s must be valid utf-8", name)
		}
	}
	return nil
}

// ComputeDestructiveScopeSHA256 returns the SHA-256 of the canonical destructive
// scope. Callers must supply a nonempty, duplicate-free target list already
// sorted bytewise by (kind, value).
func ComputeDestructiveScopeSHA256(requestID string, targets []DestructiveTarget, impact string) (string, error) {
	if err := requireNonEmptyUTF8("request_id", requestID); err != nil {
		return "", err
	}
	if err := requireNonEmptyUTF8("impact", impact); err != nil {
		return "", err
	}
	if err := validateDestructiveTargets(targets); err != nil {
		return "", err
	}

	var canonical bytes.Buffer
	canonical.WriteString(`{"impact":`)
	writeCanonicalJSONString(&canonical, impact)
	canonical.WriteString(`,"request_id":`)
	writeCanonicalJSONString(&canonical, requestID)
	canonical.WriteString(`,"scope_version":1,"targets":[`)
	for index, target := range targets {
		if index > 0 {
			canonical.WriteByte(',')
		}
		canonical.WriteString(`{"kind":`)
		writeCanonicalJSONString(&canonical, string(target.Kind))
		canonical.WriteString(`,"value":`)
		writeCanonicalJSONString(&canonical, target.Value)
		canonical.WriteByte('}')
	}
	canonical.WriteString(`]}`)
	scopeHash := sha256.Sum256(canonical.Bytes())
	return fmt.Sprintf("sha256:%x", scopeHash), nil
}

// NormalizeDestructiveRequest validates a host request and returns an owned
// copy. It deliberately rejects unsorted or duplicate targets instead of
// silently changing the scope that the host presented.
func NormalizeDestructiveRequest(request DestructiveRequest) (DestructiveRequest, error) {
	if !validSHA256(request.ScopeSHA256) {
		return DestructiveRequest{}, errors.New("destructive_request.scope_sha256 must use lowercase sha256:<64 hex> format")
	}
	scopeHash, err := ComputeDestructiveScopeSHA256(request.RequestID, request.Targets, request.Impact)
	if err != nil {
		return DestructiveRequest{}, fmt.Errorf("invalid destructive_request: %w", err)
	}
	if request.ScopeSHA256 != scopeHash {
		return DestructiveRequest{}, errors.New("destructive_request.scope_sha256 does not match its canonical scope")
	}
	request.Targets = append([]DestructiveTarget(nil), request.Targets...)
	return request, nil
}

func validateDestructiveAuthorization(authorization DestructiveAuthorization) error {
	if authorization.ScopeVersion != DestructiveScopeVersion {
		return fmt.Errorf("destructive_authorization.scope_version must be %d", DestructiveScopeVersion)
	}
	if err := requireNonEmptyUTF8("destructive_authorization.originating_action_id", authorization.OriginatingActionID); err != nil {
		return err
	}
	if !validSHA256(authorization.ScopeSHA256) {
		return errors.New("destructive_authorization.scope_sha256 must use lowercase sha256:<64 hex> format")
	}
	scopeHash, err := ComputeDestructiveScopeSHA256(authorization.RequestID, authorization.Targets, authorization.Impact)
	if err != nil {
		return fmt.Errorf("invalid destructive_authorization: %w", err)
	}
	if authorization.ScopeSHA256 != scopeHash {
		return errors.New("destructive_authorization.scope_sha256 does not match its canonical scope")
	}
	if !utf8.ValidString(authorization.ConfirmedAt) {
		return errors.New("destructive_authorization.confirmed_at must be valid utf-8")
	}
	if !strings.HasSuffix(authorization.ConfirmedAt, "Z") {
		return errors.New("destructive_authorization.confirmed_at must use UTC Z notation")
	}
	confirmedAt, err := time.Parse(time.RFC3339, authorization.ConfirmedAt)
	if err != nil {
		return errors.New("destructive_authorization.confirmed_at must be RFC3339")
	}
	_, offset := confirmedAt.Zone()
	if offset != 0 {
		return errors.New("destructive_authorization.confirmed_at must be UTC")
	}
	return nil
}

func validateDestructiveTargets(targets []DestructiveTarget) error {
	if len(targets) == 0 {
		return errors.New("targets must contain at least one target")
	}
	for index, target := range targets {
		switch target.Kind {
		case DestructiveTargetPath, DestructiveTargetGitRef, DestructiveTargetExternalResource, DestructiveTargetDataDomain:
		default:
			return fmt.Errorf("targets[%d] has unsupported kind %q", index, target.Kind)
		}
		if err := requireNonEmptyUTF8(fmt.Sprintf("targets[%d].value", index), target.Value); err != nil {
			return err
		}
		if index == 0 {
			continue
		}
		previous := targets[index-1]
		if previous.Kind == target.Kind && previous.Value == target.Value {
			return fmt.Errorf("targets[%d] duplicates targets[%d]", index, index-1)
		}
		if string(previous.Kind) > string(target.Kind) || (previous.Kind == target.Kind && previous.Value > target.Value) {
			return errors.New("targets must be bytewise sorted by kind and value")
		}
	}
	return nil
}

func writeCanonicalJSONString(builder *bytes.Buffer, value string) {
	const hexadecimal = "0123456789abcdef"
	builder.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"', '\\':
			builder.WriteByte('\\')
			builder.WriteRune(character)
		case '\b':
			builder.WriteString(`\b`)
		case '\t':
			builder.WriteString(`\t`)
		case '\n':
			builder.WriteString(`\n`)
		case '\f':
			builder.WriteString(`\f`)
		case '\r':
			builder.WriteString(`\r`)
		default:
			if character >= 0 && character <= 0x1f {
				builder.WriteString(`\u00`)
				builder.WriteByte(hexadecimal[byte(character)>>4])
				builder.WriteByte(hexadecimal[byte(character)&0x0f])
			} else {
				builder.WriteRune(character)
			}
		}
	}
	builder.WriteByte('"')
}

func DecodeOutcome(reader io.Reader) (Outcome, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maxOutcomeBytes+1))
	if err != nil {
		return Outcome{}, fmt.Errorf("read outcome: %w", err)
	}
	if len(raw) > maxOutcomeBytes {
		return Outcome{}, fmt.Errorf("decode outcome: payload exceeds %d bytes", maxOutcomeBytes)
	}
	var outcome Outcome
	if err := decodeStrictJSON(raw, &outcome); err != nil {
		return Outcome{}, fmt.Errorf("decode outcome: %w", err)
	}
	if outcome.ContractVersion != ContractVersion {
		return Outcome{}, &VersionError{Received: outcome.ContractVersion}
	}
	if err := validateOutcomeJSONShape(raw, outcome); err != nil {
		return Outcome{}, fmt.Errorf("decode outcome: %w", err)
	}
	digest := sha256.Sum256(raw)
	outcome.RawSHA256 = fmt.Sprintf("sha256:%x", digest)
	return outcome, nil
}

func validateOutcomeJSONShape(raw []byte, outcome Outcome) error {
	root, err := requiredJSONObject(raw, "$", []string{
		"contract_version", "action_id", "status", "summary", "observations", "known_issues",
		"suggested_actions", "pause", "implementation", "review",
	})
	if err != nil {
		return err
	}
	for _, field := range []string{"observations", "known_issues", "suggested_actions"} {
		if isJSONNull(root[field]) {
			return fmt.Errorf("%s must be an array, not null", field)
		}
	}

	suggestions, err := requiredJSONArray(root["suggested_actions"], "$.suggested_actions")
	if err != nil {
		return err
	}
	for index, item := range suggestions {
		if _, err := requiredJSONObject(item, fmt.Sprintf("$.suggested_actions[%d]", index), []string{"kind", "brief"}); err != nil {
			return err
		}
	}
	if outcome.Pause != nil {
		pause, err := requiredJSONObject(root["pause"], "$.pause", []string{"reason", "question", "destructive_request"})
		if err != nil {
			return err
		}
		if outcome.Pause.DestructiveRequest != nil {
			request, err := requiredJSONObject(pause["destructive_request"], "$.pause.destructive_request", []string{"request_id", "targets", "impact", "scope_sha256"})
			if err != nil {
				return err
			}
			if isJSONNull(request["targets"]) {
				return errors.New("$.pause.destructive_request.targets must be an array, not null")
			}
			targets, err := requiredJSONArray(request["targets"], "$.pause.destructive_request.targets")
			if err != nil {
				return err
			}
			for index, item := range targets {
				if _, err := requiredJSONObject(item, fmt.Sprintf("$.pause.destructive_request.targets[%d]", index), []string{"kind", "value"}); err != nil {
					return err
				}
			}
		}
	}
	if outcome.Implementation != nil {
		implementation, err := requiredJSONObject(root["implementation"], "$.implementation", []string{"result", "files_changed", "activities", "uncertainties", "attempts"})
		if err != nil {
			return err
		}
		for _, field := range []string{"files_changed", "activities", "uncertainties"} {
			if isJSONNull(implementation[field]) {
				return fmt.Errorf("$.implementation.%s must be an array, not null", field)
			}
		}
		activities, err := requiredJSONArray(implementation["activities"], "$.implementation.activities")
		if err != nil {
			return err
		}
		for index, item := range activities {
			if _, err := requiredJSONObject(item, fmt.Sprintf("$.implementation.activities[%d]", index), []string{"kind", "command", "exit_code", "summary"}); err != nil {
				return err
			}
		}
	}
	if outcome.Review != nil {
		review, err := requiredJSONObject(root["review"], "$.review", []string{"result", "findings", "uncertainties"})
		if err != nil {
			return err
		}
		for _, field := range []string{"findings", "uncertainties"} {
			if isJSONNull(review[field]) {
				return fmt.Errorf("$.review.%s must be an array, not null", field)
			}
		}
		findings, err := requiredJSONArray(review["findings"], "$.review.findings")
		if err != nil {
			return err
		}
		for index, item := range findings {
			if _, err := requiredJSONObject(item, fmt.Sprintf("$.review.findings[%d]", index), []string{"location", "summary", "detail"}); err != nil {
				return err
			}
		}
	}
	return nil
}

func requiredJSONObject(raw []byte, path string, fields []string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("%s must be an object: %w", path, err)
	}
	if object == nil {
		return nil, fmt.Errorf("%s must be an object", path)
	}
	for _, field := range fields {
		if _, exists := object[field]; !exists {
			return nil, fmt.Errorf("required field %q is missing at %s", field, path)
		}
	}
	return object, nil
}

func requiredJSONArray(raw []byte, path string) ([]json.RawMessage, error) {
	var array []json.RawMessage
	if err := json.Unmarshal(raw, &array); err != nil {
		return nil, fmt.Errorf("%s must be an array: %w", path, err)
	}
	if array == nil {
		return nil, fmt.Errorf("%s must be an array, not null", path)
	}
	return array, nil
}

func isJSONNull(raw []byte) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func (outcome Outcome) Validate(kind ActionKind, actionID string) error {
	if outcome.ContractVersion != ContractVersion {
		return &VersionError{Received: outcome.ContractVersion}
	}
	if err := requireNonEmptyUTF8("current action_id", actionID); err != nil {
		return err
	}
	if !utf8.ValidString(outcome.ActionID) || outcome.ActionID != actionID {
		return fmt.Errorf("outcome action_id %q does not match current action %q", outcome.ActionID, actionID)
	}
	if !ValidActionKind(kind) {
		return fmt.Errorf("invalid action kind %q", kind)
	}
	switch outcome.Status {
	case OutcomeCompleted, OutcomeNeedsInput, OutcomePartial, OutcomeError:
	default:
		return fmt.Errorf("invalid outcome status %q", outcome.Status)
	}
	if err := requireNonEmptyUTF8("outcome summary", outcome.Summary); err != nil {
		return err
	}
	if err := validateTextArray("observations", outcome.Observations); err != nil {
		return err
	}
	if err := validateTextArray("known_issues", outcome.KnownIssues); err != nil {
		return err
	}
	if outcome.SuggestedActions == nil {
		return errors.New("suggested_actions must be a non-null array")
	}
	if len(outcome.SuggestedActions) > 1 {
		return errors.New("suggested_actions may contain at most one item")
	}
	for index, suggested := range outcome.SuggestedActions {
		switch suggested.Kind {
		case ActionClarify, ActionImplement, ActionSummarize:
		default:
			return fmt.Errorf("suggested_actions[%d] has invalid kind %q", index, suggested.Kind)
		}
		if err := requireNonEmptyUTF8(fmt.Sprintf("suggested_actions[%d].brief", index), suggested.Brief); err != nil {
			return err
		}
		if len(suggested.Brief) > maxSuggestedActionBriefBytes {
			return fmt.Errorf("suggested_actions[%d].brief exceeds %d bytes", index, maxSuggestedActionBriefBytes)
		}
	}

	if outcome.Status == OutcomeNeedsInput {
		if outcome.Pause == nil {
			return errors.New("needs_input requires pause")
		}
		if len(outcome.SuggestedActions) != 0 {
			return errors.New("needs_input requires an empty suggested_actions array")
		}
		if err := validatePause(kind, *outcome.Pause); err != nil {
			return err
		}
	} else if outcome.Pause != nil {
		return errors.New("pause is only valid with needs_input")
	}

	if kind != ActionImplement && outcome.Implementation != nil {
		return errors.New("implementation is only valid for implement actions")
	}
	if kind != ActionReview && outcome.Review != nil {
		return errors.New("review is only valid for review actions")
	}

	switch kind {
	case ActionOrient:
		return validateOrientOutcome(outcome)
	case ActionClarify:
		return validateClarifyOutcome(outcome)
	case ActionImplement:
		return validateImplementOutcome(outcome)
	case ActionReview:
		return validateReviewOutcome(outcome)
	case ActionSummarize:
		return validateSummarizeOutcome(outcome)
	default:
		return fmt.Errorf("invalid action kind %q", kind)
	}
}

func validatePause(kind ActionKind, pause Pause) error {
	if err := requireNonEmptyUTF8("pause.question", pause.Question); err != nil {
		return err
	}
	switch pause.Reason {
	case PauseDecisionRequired, PauseEnvironmentUnavailable:
		if pause.DestructiveRequest != nil {
			return errors.New("pause.destructive_request is only valid for destructive_confirmation_required")
		}
	case PauseDestructiveConfirm:
		if kind != ActionImplement {
			return errors.New("destructive_confirmation_required is only valid for implement actions")
		}
		if pause.DestructiveRequest == nil {
			return errors.New("destructive_confirmation_required requires destructive_request")
		}
		if _, err := NormalizeDestructiveRequest(*pause.DestructiveRequest); err != nil {
			return err
		}
	default:
		return fmt.Errorf("needs_input requires a supported host pause reason, got %q", pause.Reason)
	}
	return nil
}

func validateOrientOutcome(outcome Outcome) error {
	if outcome.Implementation != nil || outcome.Review != nil {
		return errors.New("orient outcomes cannot include implementation or review")
	}
	switch outcome.Status {
	case OutcomeCompleted, OutcomePartial, OutcomeError:
		return nil
	case OutcomeNeedsInput:
		if outcome.Pause.Reason == PauseDestructiveConfirm {
			return errors.New("orient cannot request destructive confirmation")
		}
		return nil
	default:
		return fmt.Errorf("orient does not support status %q", outcome.Status)
	}
}

func validateClarifyOutcome(outcome Outcome) error {
	if outcome.Implementation != nil || outcome.Review != nil {
		return errors.New("clarify outcomes cannot include implementation or review")
	}
	switch outcome.Status {
	case OutcomeCompleted, OutcomeError:
		return nil
	case OutcomeNeedsInput:
		if outcome.Pause.Reason == PauseDestructiveConfirm {
			return errors.New("clarify cannot request destructive confirmation")
		}
		return nil
	default:
		return fmt.Errorf("clarify does not support status %q", outcome.Status)
	}
}

func validateImplementOutcome(outcome Outcome) error {
	if outcome.Review != nil {
		return errors.New("implement outcomes cannot include review")
	}
	if len(outcome.SuggestedActions) != 0 {
		return errors.New("implement outcomes cannot suggest actions")
	}
	if outcome.Status == OutcomeNeedsInput {
		if outcome.Implementation != nil {
			return errors.New("needs_input implement outcome requires implementation null")
		}
		return nil
	}
	if outcome.Implementation == nil {
		return errors.New("non-paused implement outcome requires implementation")
	}
	if err := validateImplementation(*outcome.Implementation); err != nil {
		return err
	}
	switch outcome.Status {
	case OutcomeCompleted:
		if outcome.Implementation.Result != ImplementationApplied && outcome.Implementation.Result != ImplementationNotNeeded {
			return errors.New("completed implement result must be applied or not_needed")
		}
	case OutcomePartial:
		if outcome.Implementation.Result != ImplementationPartial {
			return errors.New("partial implement result must be partial")
		}
	case OutcomeError:
		if outcome.Implementation.Result != ImplementationUnable {
			return errors.New("error implement result must be unable")
		}
	default:
		return fmt.Errorf("implement does not support status %q", outcome.Status)
	}
	return nil
}

func validateImplementation(implementation Implementation) error {
	switch implementation.Result {
	case ImplementationApplied, ImplementationPartial, ImplementationNotNeeded, ImplementationUnable:
	default:
		return fmt.Errorf("implementation has unsupported result %q", implementation.Result)
	}
	if implementation.Attempts <= 0 {
		return errors.New("implementation.attempts must be positive")
	}
	if err := validateTextArray("implementation.files_changed", implementation.FilesChanged); err != nil {
		return err
	}
	if err := validateTextArray("implementation.uncertainties", implementation.Uncertainties); err != nil {
		return err
	}
	if implementation.Activities == nil {
		return errors.New("implementation.activities must be a non-null array")
	}
	for index, activity := range implementation.Activities {
		switch activity.Kind {
		case "test", "typecheck", "build", "lint":
		default:
			return fmt.Errorf("implementation.activities[%d] has unsupported kind %q", index, activity.Kind)
		}
		if err := requireNonEmptyUTF8(fmt.Sprintf("implementation.activities[%d].command", index), activity.Command); err != nil {
			return err
		}
		if err := requireNonEmptyUTF8(fmt.Sprintf("implementation.activities[%d].summary", index), activity.Summary); err != nil {
			return err
		}
		if activity.ExitCode < 0 {
			return fmt.Errorf("implementation.activities[%d].exit_code cannot be negative", index)
		}
	}
	return nil
}

func validateReviewOutcome(outcome Outcome) error {
	if outcome.Implementation != nil {
		return errors.New("review outcomes cannot include implementation")
	}
	if len(outcome.SuggestedActions) != 0 {
		return errors.New("review outcomes cannot suggest actions")
	}
	if outcome.Status == OutcomeNeedsInput {
		return errors.New("review does not support needs_input")
	}
	if outcome.Review == nil {
		return errors.New("review outcome requires review")
	}
	if err := validateReview(*outcome.Review); err != nil {
		return err
	}
	switch outcome.Status {
	case OutcomeCompleted:
		if outcome.Review.Result != ReviewNoFindings && outcome.Review.Result != ReviewFindings {
			return errors.New("completed review result must be no_findings_reported or findings_reported")
		}
	case OutcomePartial:
		if outcome.Review.Result != ReviewInconclusive {
			return errors.New("partial review result must be inconclusive")
		}
	case OutcomeError:
		if outcome.Review.Result != ReviewError {
			return errors.New("error review result must be error")
		}
	default:
		return fmt.Errorf("review does not support status %q", outcome.Status)
	}
	return nil
}

func validateReview(review Review) error {
	if review.Findings == nil {
		return errors.New("review.findings must be a non-null array")
	}
	if err := validateTextArray("review.uncertainties", review.Uncertainties); err != nil {
		return err
	}
	for index, finding := range review.Findings {
		if err := requireNonEmptyUTF8(fmt.Sprintf("review.findings[%d].location", index), finding.Location); err != nil {
			return err
		}
		if err := requireNonEmptyUTF8(fmt.Sprintf("review.findings[%d].summary", index), finding.Summary); err != nil {
			return err
		}
		if err := requireNonEmptyUTF8(fmt.Sprintf("review.findings[%d].detail", index), finding.Detail); err != nil {
			return err
		}
	}
	if review.Result == ReviewFindings && len(review.Findings) == 0 {
		return errors.New("findings_reported requires at least one finding")
	}
	if review.Result == ReviewNoFindings && len(review.Findings) != 0 {
		return errors.New("no_findings_reported requires zero findings")
	}
	return nil
}

func validateSummarizeOutcome(outcome Outcome) error {
	if outcome.Implementation != nil || outcome.Review != nil {
		return errors.New("summarize outcomes cannot include implementation or review")
	}
	if len(outcome.SuggestedActions) != 0 {
		return errors.New("summarize outcomes cannot suggest actions")
	}
	if outcome.Status != OutcomeCompleted && outcome.Status != OutcomeError {
		return fmt.Errorf("summarize does not support status %q", outcome.Status)
	}
	return nil
}

func validateTextArray(name string, values []string) error {
	if values == nil {
		return fmt.Errorf("%s must be a non-null array", name)
	}
	for index, value := range values {
		if !utf8.ValidString(value) {
			return fmt.Errorf("%s[%d] must be valid utf-8", name, index)
		}
	}
	return nil
}

func requireNonEmptyUTF8(name, value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s must be valid utf-8", name)
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, character := range value[len("sha256:"):] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
