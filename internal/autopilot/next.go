package autopilot

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/signalridge/slipway/internal/runstore"
)

// NextOperation identifies the kind of protocol mutation that can move a Run
// forward. It is machine authority; display commands are derived outside this
// package from a fully resolved argv.
type NextOperation string

const (
	NextOperationAction  NextOperation = "action"
	NextOperationAnswer  NextOperation = "answer"
	NextOperationResume  NextOperation = "resume"
	NextOperationStart   NextOperation = "start"
	NextOperationCommand NextOperation = "command"
	NextOperationNone    NextOperation = "none"
)

// NextInputType describes one value that a host must append to a variant.
type NextInputType string

const (
	NextInputString NextInputType = "string"
	NextInputPath   NextInputType = "path"
	NextInputEnum   NextInputType = "enum"
	NextInputDigest NextInputType = "digest"
)

// Next is the structured recovery authority for a non-Action response.
type Next struct {
	Operation         NextOperation `json:"operation"`
	WorkspaceIdentity string        `json:"workspace_identity"`
	Variants          []NextVariant `json:"variants"`
	workspaceRoot     string
}

// NextVariant is one complete fixed argv plus its ordered typed inputs.
type NextVariant struct {
	ID       string      `json:"id"`
	BaseArgv []string    `json:"base_argv"`
	Inputs   []NextInput `json:"inputs"`
}

// NextInput describes one flag/value pair appended during resolution.
type NextInput struct {
	Name     string        `json:"name"`
	Type     NextInputType `json:"type"`
	Flag     string        `json:"flag"`
	Required bool          `json:"required"`
	Choices  []string      `json:"choices,omitempty"`
}

// NextInputValue is a typed, uninterpreted input supplied by a host.
type NextInputValue struct {
	Type  NextInputType
	Value string
}

var (
	nextIDPattern    = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	nextNamePattern  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	nextFlagPattern  = regexp.MustCompile(`^--[a-z][a-z0-9-]*$`)
	pseudoValueNames = map[string]struct{}{
		"<answer>": {},
		"<file>":   {},
	}
)

// Validate checks that Next can be expanded without shell interpretation.
func (next Next) Validate() error {
	switch next.Operation {
	case NextOperationAction, NextOperationAnswer, NextOperationResume, NextOperationStart, NextOperationCommand, NextOperationNone:
	default:
		return fmt.Errorf("next.operation %q is unsupported", next.Operation)
	}
	if !validSHA256(next.WorkspaceIdentity) {
		return errors.New("next.workspace_identity must be a lowercase sha256 digest")
	}
	if next.Variants == nil {
		return errors.New("next.variants must be a non-null array")
	}
	workspaceRoot := next.workspaceRoot
	if workspaceRoot != "" {
		if err := validateWorkspaceRoot(workspaceRoot); err != nil {
			return err
		}
	}
	if next.Operation == NextOperationNone {
		if len(next.Variants) != 0 {
			return errors.New("next operation none cannot contain variants")
		}
		return nil
	}
	if len(next.Variants) == 0 {
		return errors.New("next requires at least one variant")
	}
	if workspaceRoot == "" {
		var err error
		workspaceRoot, err = variantWorkspaceRoot(next.Variants[0], 0)
		if err != nil {
			return err
		}
	}

	variantIDs := make(map[string]struct{}, len(next.Variants))
	for index, variant := range next.Variants {
		if !nextIDPattern.MatchString(variant.ID) {
			return fmt.Errorf("next.variants[%d].id must be nonempty lower-case kebab-case", index)
		}
		if _, exists := variantIDs[variant.ID]; exists {
			return fmt.Errorf("next variant id %q is duplicated", variant.ID)
		}
		variantIDs[variant.ID] = struct{}{}
		if err := validateNextVariant(workspaceRoot, variant, index); err != nil {
			return err
		}
		if err := validateNextOperationFamily(next.Operation, variant); err != nil {
			return fmt.Errorf("next.variants[%d]: %w", index, err)
		}
	}
	return nil
}

// validateNextOperationFamily couples each Next operation to the argv grammar
// its variants may carry. The coarse operation label is advertised as semantic
// authority, so it must agree with base_argv instead of letting any producer
// label, for example, a read-only status retry as operation "resume".
// validateNextOperationFamily couples each Next operation to the argv grammar
// its variants may carry. The coarse operation label is advertised as semantic
// authority, so it must agree with base_argv instead of letting any producer
// label, for example, a read-only status retry as operation "resume".
//
// The skip-action variant is a universal escape hatch available on every waiting
// Action regardless of operation, so it is exempt from the grammar check.
func validateNextOperationFamily(operation NextOperation, variant NextVariant) error {
	if variant.ID == "skip-action" {
		return nil
	}
	argv := variant.BaseArgv
	switch operation {
	case NextOperationAction:
		if len(argv) < 4 || argv[1] != "_machine" || argv[2] != "submit" {
			return errors.New("operation action requires base_argv slipway _machine submit ...")
		}
	case NextOperationAnswer:
		if len(argv) < 4 || argv[1] != "_machine" || argv[2] != "answer" {
			return errors.New("operation answer requires base_argv slipway _machine answer ...")
		}
	case NextOperationResume:
		if len(argv) < 3 || argv[1] != "_machine" || argv[2] != "resume" {
			return errors.New("operation resume requires base_argv slipway _machine resume ...")
		}
	case NextOperationStart:
		if len(argv) < 2 || argv[1] != "run" {
			return errors.New("operation start requires base_argv slipway run ...")
		}
	case NextOperationCommand:
		if len(argv) < 2 {
			return errors.New("operation command requires a nonempty slipway base_argv")
		}
		// Command covers read-only or advisory recovery commands that are not a
		// Run mutation (status, doctor, list, install --refresh). It must never
		// carry the _machine/submit/answer/resume grammar owned by other ops.
		if argv[1] == "_machine" {
			return errors.New("operation command must not carry _machine mutation grammar")
		}
	case NextOperationNone:
		// Validated above; no variant reaches here.
	}
	return nil
}

func validateNextVariant(workspaceRoot string, variant NextVariant, variantIndex int) error {
	if len(variant.BaseArgv) == 0 {
		return fmt.Errorf("next.variants[%d].base_argv must be a nonempty array", variantIndex)
	}
	if variant.Inputs == nil {
		return fmt.Errorf("next.variants[%d].inputs must be a non-null array", variantIndex)
	}
	root, err := variantWorkspaceRoot(variant, variantIndex)
	if err != nil {
		return err
	}
	if root != workspaceRoot {
		return fmt.Errorf("next.variants[%d].base_argv must preserve the exact workspace root after --root", variantIndex)
	}
	for index, argument := range variant.BaseArgv {
		if !utf8.ValidString(argument) || argument == "" || strings.IndexByte(argument, 0) >= 0 {
			return fmt.Errorf("next.variants[%d].base_argv[%d] must be nonempty valid utf-8 without NUL", variantIndex, index)
		}
		if isPseudoValue(argument) {
			return fmt.Errorf("next.variants[%d].base_argv[%d] cannot contain a placeholder", variantIndex, index)
		}
	}
	if variant.BaseArgv[0] != "slipway" {
		return fmt.Errorf("next.variants[%d].base_argv must begin with slipway", variantIndex)
	}

	inputNames := make(map[string]struct{}, len(variant.Inputs))
	inputFlags := make(map[string]struct{}, len(variant.Inputs))
	for index, input := range variant.Inputs {
		if !nextNamePattern.MatchString(input.Name) {
			return fmt.Errorf("next.variants[%d].inputs[%d].name must be nonempty lower-case snake-case", variantIndex, index)
		}
		if _, exists := inputNames[input.Name]; exists {
			return fmt.Errorf("next input name %q is duplicated in variant %q", input.Name, variant.ID)
		}
		inputNames[input.Name] = struct{}{}
		if !nextFlagPattern.MatchString(input.Flag) {
			return fmt.Errorf("next input %q has invalid flag %q", input.Name, input.Flag)
		}
		if _, exists := inputFlags[input.Flag]; exists {
			return fmt.Errorf("next input flag %q is duplicated in variant %q", input.Flag, variant.ID)
		}
		inputFlags[input.Flag] = struct{}{}
		if containsString(variant.BaseArgv, input.Flag) {
			return fmt.Errorf("next input flag %q is already present in base_argv", input.Flag)
		}
		switch input.Type {
		case NextInputString, NextInputPath, NextInputDigest:
			if input.Choices != nil {
				return fmt.Errorf("next input %q only permits choices for enum type", input.Name)
			}
		case NextInputEnum:
			if len(input.Choices) == 0 {
				return fmt.Errorf("next enum input %q requires nonempty choices", input.Name)
			}
			seenChoices := make(map[string]struct{}, len(input.Choices))
			for _, choice := range input.Choices {
				if !utf8.ValidString(choice) || choice == "" {
					return fmt.Errorf("next enum input %q has an invalid choice", input.Name)
				}
				if _, exists := seenChoices[choice]; exists {
					return fmt.Errorf("next enum input %q has duplicate choice %q", input.Name, choice)
				}
				seenChoices[choice] = struct{}{}
			}
		default:
			return fmt.Errorf("next input %q has unsupported type %q", input.Name, input.Type)
		}
	}
	return nil
}

// Resolve expands one variant by appending supplied inputs in schema order.
// Values remain individual argv elements and receive no shell interpretation.
func (next Next) Resolve(variantID string, values map[string]NextInputValue) ([]string, error) {
	if err := next.Validate(); err != nil {
		return nil, err
	}
	var selected *NextVariant
	for index := range next.Variants {
		if next.Variants[index].ID == variantID {
			selected = &next.Variants[index]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("unknown next variant %q", variantID)
	}
	for name := range values {
		known := false
		for _, input := range selected.Inputs {
			if input.Name == name {
				known = true
				break
			}
		}
		if !known {
			return nil, fmt.Errorf("unknown input %q for next variant %q", name, variantID)
		}
	}

	resolved := append([]string(nil), selected.BaseArgv...)
	for _, input := range selected.Inputs {
		value, supplied := values[input.Name]
		if !supplied {
			if input.Required {
				return nil, fmt.Errorf("required input %q is missing", input.Name)
			}
			continue
		}
		if value.Type != input.Type {
			return nil, fmt.Errorf("input %q requires type %s", input.Name, input.Type)
		}
		if !utf8.ValidString(value.Value) || value.Value == "" || strings.IndexByte(value.Value, 0) >= 0 {
			return nil, fmt.Errorf("input %q must be nonempty valid utf-8 without NUL", input.Name)
		}
		switch input.Type {
		case NextInputEnum:
			if !containsString(input.Choices, value.Value) {
				return nil, fmt.Errorf("input %q must be one of %s", input.Name, strings.Join(input.Choices, ", "))
			}
		case NextInputDigest:
			if !validSHA256(value.Value) {
				return nil, fmt.Errorf("input %q must use lowercase sha256:<64 hex> format", input.Name)
			}
		}
		resolved = append(resolved, input.Flag, value.Value)
	}
	return resolved, nil
}

// NewCommandNext constructs and validates one command-oriented Next value.
func variantWorkspaceRoot(variant NextVariant, variantIndex int) (string, error) {
	rootCount := 0
	root := ""
	for index, argument := range variant.BaseArgv {
		if argument != "--root" {
			continue
		}
		rootCount++
		if index+1 >= len(variant.BaseArgv) {
			return "", fmt.Errorf("next.variants[%d].base_argv requires a value after --root", variantIndex)
		}
		root = variant.BaseArgv[index+1]
	}
	if rootCount != 1 {
		return "", fmt.Errorf("next.variants[%d].base_argv must contain exactly one --root", variantIndex)
	}
	if err := validateWorkspaceRoot(root); err != nil {
		return "", fmt.Errorf("next.variants[%d]: %w", variantIndex, err)
	}
	return root, nil
}

func validateWorkspaceRoot(root string) error {
	if root == "" || !utf8.ValidString(root) || strings.IndexByte(root, 0) >= 0 || !filepath.IsAbs(root) {
		return errors.New("next workspace root must be an absolute valid utf-8 path without NUL")
	}
	return nil
}

func workspaceIdentityForNext(workspaceRoot string) (string, string) {
	if identity, err := runstore.DiscoverWorkspaceIdentity(workspaceRoot); err == nil {
		return identity.ID, identity.WorktreeRoot
	}
	absolute, err := filepath.Abs(workspaceRoot)
	if err == nil {
		workspaceRoot = filepath.Clean(absolute)
	}
	digest := sha256.Sum256([]byte("unavailable-workspace-root-v1\x00" + workspaceRoot))
	return "sha256:" + hex.EncodeToString(digest[:]), workspaceRoot
}

func NewCommandNext(operation NextOperation, workspaceRoot, variantID string, baseArgv []string, inputs []NextInput) (Next, error) {
	workspaceIdentity, canonicalRoot := workspaceIdentityForNext(workspaceRoot)
	copiedArgv := make([]string, len(baseArgv))
	copy(copiedArgv, baseArgv)
	copiedInputs := make([]NextInput, len(inputs))
	copy(copiedInputs, inputs)
	next := Next{
		Operation:         operation,
		WorkspaceIdentity: workspaceIdentity,
		Variants: []NextVariant{{
			ID:       variantID,
			BaseArgv: copiedArgv,
			Inputs:   copiedInputs,
		}},
		workspaceRoot: canonicalRoot,
	}
	if err := next.Validate(); err != nil {
		return Next{}, err
	}
	return next, nil
}

// NoneNext returns a terminal structured Next value for an absolute workspace.
func NoneNext(workspace string) Next {
	identity := workspace
	root := ""
	if !validSHA256(identity) {
		identity, root = workspaceIdentityForNext(workspace)
	}
	return Next{Operation: NextOperationNone, WorkspaceIdentity: identity, Variants: []NextVariant{}, workspaceRoot: root}
}

// DeriveNext deterministically projects the current Run recovery authority.
func DeriveNext(run Run) (Next, error) {
	switch {
	case run.State == RunEnded:
		return validatedNext(NoneNext(run.WorkspaceIdentity.ID))
	case run.SourceCandidate != nil:
		return sourceCandidateNext(run)
	case run.State == RunActive && run.CurrentAction != nil:
		return actionNext(run)
	case run.State == RunPaused && run.PauseReason == PauseDecisionRequired && run.CurrentAction != nil:
		return decisionNext(run)
	case run.State == RunPaused && run.PauseReason == PauseDestructiveConfirm:
		return destructiveNext(run)
	case run.State == RunPaused && run.PauseReason == PauseEnvironmentUnavailable && run.CurrentAction != nil:
		return environmentNext(run)
	default:
		return resumeNext(run)
	}
}

// DeriveResumeNext projects only safe resume choices, even when a caller made
// an invalid resume attempt while another Action remained current.
func DeriveResumeNext(run Run) (Next, error) {
	if run.State == RunEnded {
		return validatedNext(NoneNext(run.WorkspaceIdentity.ID))
	}
	if run.SourceCandidate != nil {
		return sourceCandidateNext(run)
	}
	return resumeNext(run)
}

func actionNext(run Run) (Next, error) {
	actionID := run.CurrentAction.ActionID
	fixed := []string{"slipway", "_machine", "submit", "--run", run.ID, "--action", actionID, "--root", run.Workspace}
	next := Next{
		Operation:         NextOperationAction,
		WorkspaceIdentity: run.WorkspaceIdentity.ID,
		workspaceRoot:     run.Workspace,
		Variants: []NextVariant{
			{
				ID:       "submit-outcome-file",
				BaseArgv: fixed,
				Inputs:   []NextInput{{Name: "outcome_file", Type: NextInputPath, Flag: "--outcome-file", Required: true}},
			},
			{
				ID:       "submit-outcome-stdin",
				BaseArgv: append(append([]string(nil), fixed...), "--outcome-stdin"),
				Inputs:   []NextInput{},
			},
			skipActionVariant(run),
		},
	}
	return validatedNext(next)
}

func skipActionVariant(run Run) NextVariant {
	return NextVariant{
		ID:       "skip-action",
		BaseArgv: []string{"slipway", "_machine", "skip", "--run", run.ID, "--action", run.CurrentAction.ActionID, "--root", run.Workspace},
		Inputs:   []NextInput{},
	}
}

func environmentNext(run Run) (Next, error) {
	next, err := resumeNext(run)
	if err != nil {
		return Next{}, err
	}
	next.Variants = append(next.Variants, skipActionVariant(run))
	return validatedNext(next)
}

func decisionNext(run Run) (Next, error) {
	next := Next{
		Operation:         NextOperationAnswer,
		WorkspaceIdentity: run.WorkspaceIdentity.ID,
		workspaceRoot:     run.Workspace,
		Variants: []NextVariant{
			{
				ID:       "answer-decision",
				BaseArgv: []string{"slipway", "_machine", "answer", "--run", run.ID, "--action", run.CurrentAction.ActionID, "--root", run.Workspace},
				Inputs:   []NextInput{{Name: "text", Type: NextInputString, Flag: "--text", Required: true}},
			},
			skipActionVariant(run),
		},
	}
	return validatedNext(next)
}

func destructiveNext(run Run) (Next, error) {
	if run.CurrentAction == nil || run.PendingDestructiveRequest == nil {
		return Next{}, errors.New("destructive pause is missing its current action or request")
	}
	request := run.PendingDestructiveRequest
	next := Next{
		Operation:         NextOperationAnswer,
		WorkspaceIdentity: run.WorkspaceIdentity.ID,
		workspaceRoot:     run.Workspace,
		Variants: []NextVariant{
			{
				ID: "confirm-destructive",
				BaseArgv: []string{
					"slipway", "_machine", "answer", "--run", run.ID, "--action", run.CurrentAction.ActionID,
					"--root", run.Workspace, "--confirm-destructive", "--scope-sha256", request.ScopeSHA256,
				},
				Inputs: []NextInput{{Name: "text", Type: NextInputString, Flag: "--text", Required: false}},
			},
			{
				ID:       "decline-or-feedback",
				BaseArgv: []string{"slipway", "_machine", "answer", "--run", run.ID, "--action", run.CurrentAction.ActionID, "--root", run.Workspace},
				Inputs:   []NextInput{{Name: "text", Type: NextInputString, Flag: "--text", Required: true}},
			},
			skipActionVariant(run),
		},
	}
	return validatedNext(next)
}

func resumeNext(run Run) (Next, error) {
	base := []string{"slipway", "_machine", "resume", run.ID, "--root", run.Workspace}
	next := Next{Operation: NextOperationResume, WorkspaceIdentity: run.WorkspaceIdentity.ID, workspaceRoot: run.Workspace}
	if run.PinnedSource == nil {
		next.Variants = []NextVariant{{ID: "resume-ad-hoc", BaseArgv: base, Inputs: []NextInput{}}}
	} else {
		next.Variants = []NextVariant{
			{
				ID:       "refresh-source",
				BaseArgv: base,
				Inputs:   []NextInput{{Name: "source_file", Type: NextInputPath, Flag: "--source-file", Required: true}},
			},
			{
				ID:       "use-pinned-source",
				BaseArgv: append(append([]string(nil), base...), "--use-pinned-source"),
				Inputs:   []NextInput{},
			},
		}
	}
	return validatedNext(next)
}

func sourceCandidateNext(run Run) (Next, error) {
	candidate := run.SourceCandidate
	base := []string{"slipway", "_machine", "resume", run.ID, "--root", run.Workspace}
	next := Next{
		Operation:         NextOperationResume,
		WorkspaceIdentity: run.WorkspaceIdentity.ID,
		workspaceRoot:     run.Workspace,
		Variants: []NextVariant{{
			ID:       "keep-pinned",
			BaseArgv: append(append([]string(nil), base...), "--source-choice", "pinned", "--candidate", candidate.CandidateID),
			Inputs:   []NextInput{},
		}},
	}
	if candidate.Valid {
		next.Variants = append(next.Variants, NextVariant{
			ID:       "adopt",
			BaseArgv: append(append([]string(nil), base...), "--source-choice", "adopt", "--candidate", candidate.CandidateID),
			Inputs:   []NextInput{},
		})
	}
	return validatedNext(next)
}

func validatedNext(next Next) (Next, error) {
	if err := next.Validate(); err != nil {
		return Next{}, err
	}
	return next, nil
}

func isPseudoValue(value string) bool {
	trimmed := strings.Trim(strings.TrimSpace(value), `"'`)
	if trimmed == "FILE" {
		return true
	}
	normalized := strings.ToLower(trimmed)
	if _, exists := pseudoValueNames[normalized]; exists {
		return true
	}
	return strings.HasPrefix(normalized, "<") && strings.HasSuffix(normalized, ">")
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
