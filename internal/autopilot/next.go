package autopilot

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
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

// NextInputType describes one typed value that a host can supply.
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

// NextInput describes one flag/value pair applied during resolution.
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

// WorkspaceRoot returns the absolute workspace root this Next targets, or
// the empty string when the Next is a terminal `none` without a root. It is a
// read-only accessor used by rendering and error-recovery helpers.
func (next Next) WorkspaceRoot() string {
	return next.workspaceRoot
}

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
//
// The skip-action variant is a universal escape hatch available on every waiting
// Action regardless of operation, so it is exempt from the grammar check.
func validateNextOperationFamily(operation NextOperation, variant NextVariant) error {
	argv := variant.BaseArgv
	if variant.ID == "skip-action" {
		if operation != NextOperationAction && operation != NextOperationAnswer && operation != NextOperationResume {
			return errors.New("skip-action is only valid for action, answer, or resume operations")
		}
		if len(argv) != 9 || argv[0] != "slipway" || argv[1] != "_machine" || argv[2] != "skip" ||
			argv[3] != "--run" || strings.HasPrefix(argv[4], "--") ||
			argv[5] != "--action" || strings.HasPrefix(argv[6], "--") || argv[7] != "--root" {
			return errors.New("skip-action requires exact slipway _machine skip --run RUN --action ACTION --root ROOT grammar")
		}
		if len(variant.Inputs) != 0 {
			return errors.New("skip-action cannot require inputs")
		}
		return nil
	}

	switch operation {
	case NextOperationAction:
		if len(argv) < 9 || argv[0] != "slipway" || argv[1] != "_machine" || argv[2] != "submit" ||
			argv[3] != "--run" || strings.HasPrefix(argv[4], "--") ||
			argv[5] != "--action" || strings.HasPrefix(argv[6], "--") || argv[7] != "--root" {
			return errors.New("operation action requires exact slipway _machine submit --run RUN --action ACTION --root ROOT [--outcome-file FILE | --outcome-stdin] grammar")
		}
		baseOptions, inputOptions, err := validateNextOperationOptions(
			"action", variant, 9,
			[]string{"--outcome-file"},
			[]string{"--outcome-stdin"},
			[]string{"--outcome-file"},
		)
		if err != nil {
			return err
		}
		_, outcomeFileInBase := baseOptions["--outcome-file"]
		outcomeFileInput, outcomeFileInInputs := inputOptions["--outcome-file"]
		_, outcomeStdin := baseOptions["--outcome-stdin"]
		if outcomeFileInInputs && (!outcomeFileInput.Required || outcomeFileInput.Type != NextInputPath) {
			return errors.New("operation action requires --outcome-file to be a required path input")
		}
		outcomeFile := outcomeFileInBase || outcomeFileInInputs
		if outcomeFile == outcomeStdin {
			return errors.New("operation action requires exactly one of --outcome-file or --outcome-stdin")
		}
		switch variant.ID {
		case "submit-outcome-file":
			if len(argv) != 9 || outcomeFileInBase || outcomeStdin ||
				!nextInputsExactly(variant.Inputs, NextInput{Name: "outcome_file", Type: NextInputPath, Flag: "--outcome-file", Required: true}) {
				return errors.New("submit-outcome-file requires one required --outcome-file path input")
			}
		case "submit-outcome-stdin":
			if len(argv) != 10 || !outcomeStdin || len(variant.Inputs) != 0 {
				return errors.New("submit-outcome-stdin requires fixed --outcome-stdin argv without inputs")
			}
		default:
			return fmt.Errorf("operation action has unsupported variant id %q", variant.ID)
		}
	case NextOperationAnswer:
		if len(argv) < 9 || argv[0] != "slipway" || argv[1] != "_machine" || argv[2] != "answer" ||
			argv[3] != "--run" || strings.HasPrefix(argv[4], "--") ||
			argv[5] != "--action" || strings.HasPrefix(argv[6], "--") || argv[7] != "--root" {
			return errors.New("operation answer requires exact slipway _machine answer --run RUN --action ACTION --root ROOT [--text TEXT] [--confirm-destructive] [--scope-sha256 DIGEST] grammar")
		}
		baseOptions, inputOptions, err := validateNextOperationOptions(
			"answer", variant, 9,
			[]string{"--text", "--scope-sha256"},
			[]string{"--confirm-destructive"},
			[]string{"--text", "--scope-sha256"},
		)
		if err != nil {
			return err
		}
		_, confirmDestructive := baseOptions["--confirm-destructive"]
		scope, scopeInBase := baseOptions["--scope-sha256"]
		scopeInput, scopeInInputs := inputOptions["--scope-sha256"]
		if confirmDestructive != (scopeInBase || scopeInInputs) {
			return errors.New("operation answer requires --confirm-destructive and --scope-sha256 together")
		}
		if scopeInBase && !validSHA256(scope) {
			return errors.New("operation answer requires --scope-sha256 to be a lowercase sha256 digest")
		}
		if scopeInInputs && (!scopeInput.Required || scopeInput.Type != NextInputDigest) {
			return errors.New("operation answer requires --scope-sha256 to be a required digest input")
		}
		if textInput, exists := inputOptions["--text"]; exists && textInput.Type != NextInputString {
			return errors.New("operation answer requires --text to be a string input")
		}
		switch variant.ID {
		case "answer-decision", "decline-or-feedback":
			if len(argv) != 9 || confirmDestructive || scopeInBase || scopeInInputs ||
				!nextInputsExactly(variant.Inputs, NextInput{Name: "text", Type: NextInputString, Flag: "--text", Required: true}) {
				return fmt.Errorf("%s requires one required --text string input", variant.ID)
			}
		case "confirm-destructive":
			if len(argv) != 12 || !confirmDestructive || !scopeInBase || scopeInInputs ||
				!nextInputsExactly(variant.Inputs, NextInput{Name: "text", Type: NextInputString, Flag: "--text", Required: false}) {
				return errors.New("confirm-destructive requires fixed confirmation scope argv and one optional --text string input")
			}
		default:
			return fmt.Errorf("operation answer has unsupported variant id %q", variant.ID)
		}
	case NextOperationResume:
		if len(argv) < 6 || argv[0] != "slipway" || argv[1] != "_machine" || argv[2] != "resume" ||
			strings.HasPrefix(argv[3], "--") || argv[4] != "--root" {
			return errors.New("operation resume requires exact slipway _machine resume RUN --root ROOT [--budget N] [--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate CANDIDATE] grammar")
		}
		baseOptions, inputOptions, err := validateNextOperationOptions(
			"resume", variant, 6,
			[]string{"--budget", "--source-file", "--source-choice", "--candidate"},
			[]string{"--use-pinned-source"},
			[]string{"--source-file", "--source-choice", "--candidate"},
		)
		if err != nil {
			return err
		}
		_, sourceFileInBase := baseOptions["--source-file"]
		sourceFileInput, sourceFileInInputs := inputOptions["--source-file"]
		if sourceFileInInputs && (!sourceFileInput.Required || sourceFileInput.Type != NextInputPath) {
			return errors.New("operation resume requires --source-file to be a required path input")
		}
		budgetValue, budgetPresent := baseOptions["--budget"]
		if budgetPresent && !canonicalBudgetDecimal(budgetValue) {
			return errors.New("operation resume requires --budget to be a canonical positive base-10 integer no greater than 1000")
		}
		choice, sourceChoiceInBase := baseOptions["--source-choice"]
		sourceChoiceInput, sourceChoiceInInputs := inputOptions["--source-choice"]
		_, candidateInBase := baseOptions["--candidate"]
		candidateInput, candidateInInputs := inputOptions["--candidate"]
		_, usePinnedSource := baseOptions["--use-pinned-source"]

		sourceChoicePresent := sourceChoiceInBase || sourceChoiceInInputs
		candidatePresent := candidateInBase || candidateInInputs
		if sourceChoicePresent != candidatePresent {
			return errors.New("operation resume requires --source-choice and --candidate together")
		}
		if sourceChoiceInBase && choice != string(SourceChoicePinned) && choice != string(SourceChoiceAdopt) {
			return errors.New("operation resume requires --source-choice to be pinned or adopt")
		}
		if sourceChoiceInInputs {
			if !sourceChoiceInput.Required || sourceChoiceInput.Type != NextInputEnum ||
				!nextSourceChoiceInputIsBounded(sourceChoiceInput) {
				return errors.New("operation resume requires a source-choice input to be a required pinned/adopt enum")
			}
		}
		if candidateInInputs && (!candidateInput.Required || candidateInput.Type != NextInputString) {
			return errors.New("operation resume requires --candidate to be a required string input")
		}
		modeCount := 0
		if sourceFileInBase || sourceFileInInputs {
			modeCount++
		}
		if usePinnedSource {
			modeCount++
		}
		if sourceChoicePresent {
			modeCount++
		}
		if modeCount > 1 {
			return errors.New("operation resume allows only one source mode")
		}
		switch variant.ID {
		case "resume-ad-hoc":
			if len(argv) != 6 || modeCount != 0 || len(variant.Inputs) != 0 {
				return errors.New("resume-ad-hoc requires fixed argv without source mode or inputs")
			}
		case "refresh-source":
			if (len(argv) != 6 && len(argv) != 8) || (len(argv) == 8 && !budgetPresent) ||
				!sourceFileInInputs || sourceFileInBase || modeCount != 1 ||
				!nextInputsExactly(variant.Inputs, NextInput{Name: "source_file", Type: NextInputPath, Flag: "--source-file", Required: true}) {
				return errors.New("refresh-source requires one required --source-file path input")
			}
		case "use-pinned-source":
			if len(argv) != 7 || !usePinnedSource || len(variant.Inputs) != 0 {
				return errors.New("use-pinned-source requires fixed --use-pinned-source argv without inputs")
			}
		case "keep-pinned":
			if len(argv) != 10 || !sourceChoiceInBase || choice != string(SourceChoicePinned) || !candidateInBase || len(variant.Inputs) != 0 {
				return errors.New("keep-pinned requires fixed pinned source-choice and candidate argv")
			}
		case "adopt":
			if len(argv) != 10 || !sourceChoiceInBase || choice != string(SourceChoiceAdopt) || !candidateInBase || len(variant.Inputs) != 0 {
				return errors.New("adopt requires fixed adopt source-choice and candidate argv")
			}
		default:
			return fmt.Errorf("operation resume has unsupported variant id %q", variant.ID)
		}
	case NextOperationStart:
		validWithoutReview := len(argv) == 9 && argv[0] == "slipway" && argv[2] == "--budget" && argv[4] == "--json" &&
			argv[5] == "--root" && argv[7] == "--"
		validWithReviewDisabled := len(argv) == 10 && argv[0] == "slipway" && argv[2] == "--budget" && argv[4] == "--json" &&
			argv[5] == "--root" && argv[7] == "--no-review" && argv[8] == "--"
		if len(argv) < 2 || argv[1] != "run" || (!validWithoutReview && !validWithReviewDisabled) {
			return errors.New("operation start requires exact slipway run --budget N --json --root ROOT [--no-review] -- GOAL grammar")
		}
		if !canonicalBudgetDecimal(argv[3]) {
			return errors.New("operation start requires --budget to be a canonical positive base-10 integer no greater than 1000")
		}
		for _, input := range variant.Inputs {
			if input.Flag != "--source-file" {
				return fmt.Errorf("operation start inputs contain unsupported flag %q", input.Flag)
			}
			if !input.Required || input.Type != NextInputPath {
				return errors.New("operation start requires --source-file to be a required path input")
			}
		}
		switch variant.ID {
		case "retry-run":
			if len(variant.Inputs) != 0 {
				return errors.New("retry-run cannot require inputs")
			}
		case "start-with-source":
			if !nextInputsExactly(variant.Inputs, NextInput{Name: "source_file", Type: NextInputPath, Flag: "--source-file", Required: true}) {
				return errors.New("start-with-source requires one required --source-file path input")
			}
		default:
			return fmt.Errorf("operation start has unsupported variant id %q", variant.ID)
		}
	case NextOperationCommand:
		if len(argv) < 2 {
			return errors.New("operation command requires a nonempty slipway base_argv")
		}
		// Command covers read-only or advisory recovery commands that are not a
		// Run mutation (status, doctor, list, install --refresh). The run and
		// _machine grammars are owned by start and the mutation operations.
		if argv[1] == "_machine" || argv[1] == "run" {
			return errors.New("operation command must not carry run or _machine grammar")
		}
	case NextOperationNone:
		// Validated above; no variant reaches here.
	}
	return nil
}

func validateNextOperationOptions(
	family string,
	variant NextVariant,
	start int,
	baseValueFlags []string,
	baseBooleanFlags []string,
	inputValueFlags []string,
) (map[string]string, map[string]NextInput, error) {
	baseOptions := make(map[string]string)
	for index := start; index < len(variant.BaseArgv); {
		flag := variant.BaseArgv[index]
		if _, duplicate := baseOptions[flag]; duplicate {
			return nil, nil, fmt.Errorf("operation %s base_argv repeats flag %q", family, flag)
		}
		switch {
		case containsString(baseBooleanFlags, flag):
			baseOptions[flag] = "true"
			index++
		case containsString(baseValueFlags, flag):
			if index+1 >= len(variant.BaseArgv) || strings.HasPrefix(variant.BaseArgv[index+1], "--") {
				return nil, nil, fmt.Errorf("operation %s flag %q requires a value", family, flag)
			}
			baseOptions[flag] = variant.BaseArgv[index+1]
			index += 2
		default:
			return nil, nil, fmt.Errorf("operation %s base_argv contains unsupported flag %q", family, flag)
		}
	}

	inputOptions := make(map[string]NextInput, len(variant.Inputs))
	for _, input := range variant.Inputs {
		if !containsString(inputValueFlags, input.Flag) {
			if containsString(baseBooleanFlags, input.Flag) {
				return nil, nil, fmt.Errorf("operation %s boolean flag %q cannot be supplied as a typed input", family, input.Flag)
			}
			return nil, nil, fmt.Errorf("operation %s inputs contain unsupported flag %q", family, input.Flag)
		}
		if _, duplicate := baseOptions[input.Flag]; duplicate {
			return nil, nil, fmt.Errorf("operation %s flag %q is present in both base_argv and inputs", family, input.Flag)
		}
		inputOptions[input.Flag] = input
	}
	return baseOptions, inputOptions, nil
}

func nextInputsExactly(inputs []NextInput, expected ...NextInput) bool {
	if len(inputs) != len(expected) {
		return false
	}
	for index := range inputs {
		actual := inputs[index]
		want := expected[index]
		if actual.Name != want.Name || actual.Type != want.Type || actual.Flag != want.Flag ||
			actual.Required != want.Required || len(actual.Choices) != 0 {
			return false
		}
	}
	return true
}

func nextSourceChoiceInputIsBounded(input NextInput) bool {
	for _, choice := range input.Choices {
		if choice != string(SourceChoicePinned) && choice != string(SourceChoiceAdopt) {
			return false
		}
	}
	return true
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

// Resolve expands one variant with supplied inputs in schema order. Inputs are
// inserted before a `--` separator when present, so positional values remain
// positional. Values remain individual argv elements and receive no shell interpretation.
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

	resolvedInputs := make([]string, 0, len(selected.Inputs)*2)
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
		resolvedInputs = append(resolvedInputs, input.Flag, value.Value)
	}

	insertAt := len(selected.BaseArgv)
	if separator := indexOfArg(selected.BaseArgv, "--"); separator >= 0 {
		insertAt = separator
	}
	resolved := make([]string, 0, len(selected.BaseArgv)+len(resolvedInputs))
	resolved = append(resolved, selected.BaseArgv[:insertAt]...)
	resolved = append(resolved, resolvedInputs...)
	resolved = append(resolved, selected.BaseArgv[insertAt:]...)
	return resolved, nil
}

// NewCommandNext constructs and validates one command-oriented Next value.
func variantWorkspaceRoot(variant NextVariant, variantIndex int) (string, error) {
	rootCount := 0
	root := ""
	for index, argument := range variant.BaseArgv {
		if argument == "--" {
			break
		}
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
		return "", fmt.Errorf("next.variants[%d].base_argv must contain exactly one --root option before --", variantIndex)
	}
	if err := validateWorkspaceRoot(root); err != nil {
		return "", fmt.Errorf("next.variants[%d]: %w", variantIndex, err)
	}
	return root, nil
}

func canonicalBudgetDecimal(value string) bool {
	if value == "" || value[0] < '1' || value[0] > '9' {
		return false
	}
	for index := 1; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return err == nil && parsed <= maxBudget
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
		// Replay rejects unknown state and pause-reason values; keep resume as the
		// forward-compatible fallback for unmatched combinations of known values.
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

func indexOfArg(argv []string, target string) int {
	for index, argument := range argv {
		if argument == target {
			return index
		}
	}
	return -1
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
