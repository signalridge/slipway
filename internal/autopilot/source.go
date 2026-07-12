package autopilot

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	SourceVersion = 1
	ParserVersion = 1

	maxSourceFileBytes              = 256 << 10
	maxAcceptedRequirementsBytes    = 64 << 10
	maxSourceClassificationErrBytes = 1 << 10
)

const (
	changeSourceMarker = "<!-- slipway-level: change/v1 -->"
	markerIdentifier   = "slipway-level:"
)

var acceptedSectionHeadings = [...]string{
	"Outcome",
	"Requirements",
	"Acceptance examples",
	"Constraints",
	"Non-goals",
}

// SourceParent identifies an optional parent Objective for traceability only.
type SourceParent struct {
	RepositoryID string `json:"repository_id"`
	IssueID      string `json:"issue_id"`
	CanonicalURL string `json:"canonical_url"`
}

// RawSourceEnvelope is the strict, ephemeral GitHub input supplied by a host.
type RawSourceEnvelope struct {
	SourceVersion int           `json:"source_version"`
	Provider      string        `json:"provider"`
	Host          string        `json:"host"`
	RepositoryID  string        `json:"repository_id"`
	IssueID       string        `json:"issue_id"`
	IssueNumber   int           `json:"issue_number"`
	CanonicalURL  string        `json:"canonical_url"`
	UpdatedAt     string        `json:"updated_at"`
	FetchedAt     string        `json:"fetched_at"`
	Title         string        `json:"title"`
	Body          string        `json:"body"`
	Labels        []string      `json:"labels"`
	Parent        *SourceParent `json:"parent,omitempty"`
}

// AcceptedRequirements contains the exact normalized Markdown bytes selected
// from the five normative Change sections.
type AcceptedRequirements struct {
	OutcomeMarkdown            string `json:"outcome_markdown"`
	RequirementsMarkdown       string `json:"requirements_markdown"`
	AcceptanceExamplesMarkdown string `json:"acceptance_examples_markdown"`
	ConstraintsMarkdown        string `json:"constraints_markdown"`
	NonGoalsMarkdown           string `json:"non_goals_markdown"`
}

// PinnedSource is the complete source snapshot that may be persisted. It
// deliberately has no raw body, comments, timestamps, or labels.
type PinnedSource struct {
	SourceVersion        int                  `json:"source_version"`
	ParserVersion        int                  `json:"parser_version"`
	Provider             string               `json:"provider"`
	Host                 string               `json:"host"`
	RepositoryID         string               `json:"repository_id"`
	IssueID              string               `json:"issue_id"`
	IssueNumber          int                  `json:"issue_number"`
	CanonicalURL         string               `json:"canonical_url"`
	URLAliases           []string             `json:"url_aliases"`
	SourceRevision       string               `json:"source_revision"`
	RequirementsRevision string               `json:"requirements_revision"`
	Title                string               `json:"title"`
	Parent               *SourceParent        `json:"parent,omitempty"`
	AcceptedRequirements AcceptedRequirements `json:"accepted_requirements"`
}

// SourceClassification reports whether a refreshed envelope is a structurally
// valid Change. Identity and projection failures are errors, not classifications.
type SourceClassification string

const (
	SourceClassificationValid   SourceClassification = "valid"
	SourceClassificationInvalid SourceClassification = "invalid"
)

const (
	SourceClassificationValidChange              = "valid_change"
	SourceClassificationObjectiveMarker          = "objective_marker"
	SourceClassificationUnsupportedMarker        = "unsupported_marker"
	SourceClassificationMultipleMarkers          = "multiple_markers"
	SourceClassificationChangeMarkerRequired     = "change_marker_required"
	SourceClassificationAcceptedSectionMissing   = "accepted_section_missing"
	SourceClassificationAcceptedSectionDuplicate = "accepted_section_duplicate"
	SourceClassificationAcceptedSectionAmbiguous = "accepted_section_ambiguous"
	SourceClassificationRequirementsTooLarge     = "accepted_requirements_too_large"
	SourceClassificationInvalidChangeBody        = "invalid_change_body"
)

// SourceCandidateInput is the normalized, path-free result of importing a
// refreshed source envelope. Invalid bodies retain only safe identity,
// projection, revision, and classification data. Snapshot is present only for a
// valid Change source.
type SourceCandidateInput struct {
	Valid                bool                 `json:"valid"`
	Classification       SourceClassification `json:"classification"`
	ClassificationCode   string               `json:"classification_code"`
	ClassificationError  string               `json:"classification_error,omitempty"`
	SourceVersion        int                  `json:"source_version"`
	ParserVersion        int                  `json:"parser_version"`
	Provider             string               `json:"provider"`
	Host                 string               `json:"host"`
	RepositoryID         string               `json:"repository_id"`
	IssueID              string               `json:"issue_id"`
	IssueNumber          int                  `json:"issue_number"`
	CanonicalURL         string               `json:"canonical_url"`
	URLAliases           []string             `json:"url_aliases"`
	SourceRevision       string               `json:"source_revision,omitempty"`
	RequirementsRevision string               `json:"requirements_revision,omitempty"`
	Title                string               `json:"title"`
	Parent               *SourceParent        `json:"parent,omitempty"`
	Snapshot             *PinnedSource        `json:"snapshot,omitempty"`
}

// ImportSourceFile safely reads and parses one source envelope. The source
// handle is closed before any JSON or Markdown parsing begins.
func ImportSourceFile(path string) (PinnedSource, error) {
	raw, err := readSourceFile(path)
	if err != nil {
		return PinnedSource{}, err
	}
	return ParseSource(raw)
}

// ImportSourceCandidateFile safely reads a refreshed envelope exactly once.
// A body-classification failure is returned as an invalid normalized input;
// malformed JSON or identity/projection fields remain hard errors.
func ImportSourceCandidateFile(path string) (SourceCandidateInput, error) {
	raw, err := readSourceFile(path)
	if err != nil {
		return SourceCandidateInput{}, err
	}
	return ParseSourceCandidate(raw)
}

// ParseSource validates an in-memory raw envelope and returns its persistable
// pinned snapshot. New Runs reject body-classification failures outright.
func ParseSource(raw []byte) (PinnedSource, error) {
	candidate, bodyErr, err := parseSourceCandidate(raw)
	if err != nil {
		return PinnedSource{}, err
	}
	if bodyErr != nil {
		return PinnedSource{}, fmt.Errorf("parse source body: %w", bodyErr)
	}
	return clonePinnedSourceValue(*candidate.Snapshot), nil
}

// ParseSourceCandidate validates the strict envelope before classifying its
// body. The returned value never contains the raw body, labels, timestamps, or
// a source-file path.
func ParseSourceCandidate(raw []byte) (SourceCandidateInput, error) {
	candidate, _, err := parseSourceCandidate(raw)
	return candidate, err
}

func parseSourceCandidate(raw []byte) (SourceCandidateInput, error, error) {
	if len(raw) > maxSourceFileBytes {
		return SourceCandidateInput{}, nil, fmt.Errorf("parse source: payload exceeds %d bytes", maxSourceFileBytes)
	}

	var envelope RawSourceEnvelope
	if err := decodeStrictJSON(raw, &envelope); err != nil {
		return SourceCandidateInput{}, nil, fmt.Errorf("parse source: %w", err)
	}
	if err := validateRawSourceEnvelope(envelope); err != nil {
		return SourceCandidateInput{}, nil, fmt.Errorf("validate source: %w", err)
	}

	normalizedBody := normalizeLineEndings(envelope.Body)
	candidate := sourceCandidateIdentity(envelope, normalizedBody)
	accepted, bodyErr := parseAcceptedRequirements(normalizedBody)
	if bodyErr != nil {
		candidate.Classification = SourceClassificationInvalid
		candidate.ClassificationCode, candidate.ClassificationError = classifySourceBodyError(bodyErr)
		return candidate, bodyErr, nil
	}

	snapshot := PinnedSource{
		SourceVersion:        SourceVersion,
		ParserVersion:        ParserVersion,
		Provider:             envelope.Provider,
		Host:                 envelope.Host,
		RepositoryID:         envelope.RepositoryID,
		IssueID:              envelope.IssueID,
		IssueNumber:          envelope.IssueNumber,
		CanonicalURL:         envelope.CanonicalURL,
		URLAliases:           make([]string, 0),
		SourceRevision:       candidate.SourceRevision,
		RequirementsRevision: requirementsRevision(accepted),
		Title:                envelope.Title,
		Parent:               cloneSourceParent(envelope.Parent),
		AcceptedRequirements: accepted,
	}
	candidate.Valid = true
	candidate.Classification = SourceClassificationValid
	candidate.ClassificationCode = SourceClassificationValidChange
	candidate.RequirementsRevision = snapshot.RequirementsRevision
	candidate.Snapshot = clonePinnedSource(&snapshot)
	return candidate, nil, nil
}

func sourceCandidateIdentity(envelope RawSourceEnvelope, normalizedBody string) SourceCandidateInput {
	return SourceCandidateInput{
		Valid:          false,
		SourceVersion:  SourceVersion,
		ParserVersion:  ParserVersion,
		Provider:       envelope.Provider,
		Host:           envelope.Host,
		RepositoryID:   envelope.RepositoryID,
		IssueID:        envelope.IssueID,
		IssueNumber:    envelope.IssueNumber,
		CanonicalURL:   envelope.CanonicalURL,
		URLAliases:     make([]string, 0),
		SourceRevision: sourceRevision(envelope, normalizedBody),
		Title:          envelope.Title,
		Parent:         cloneSourceParent(envelope.Parent),
	}
}

func classifySourceBodyError(err error) (string, string) {
	message := err.Error()
	switch {
	case strings.Contains(message, "objective marker"):
		return SourceClassificationObjectiveMarker, "objective marker cannot be used as a change source"
	case strings.Contains(message, "unsupported slipway-level marker"):
		return SourceClassificationUnsupportedMarker, "source uses an unsupported slipway-level marker"
	case strings.Contains(message, "multiple slipway-level markers"):
		return SourceClassificationMultipleMarkers, "source contains multiple slipway-level markers outside code fences"
	case strings.Contains(message, "first nonempty body line"), strings.Contains(message, "marker must appear"):
		return SourceClassificationChangeMarkerRequired, "source must begin with one change/v1 marker outside code fences"
	case strings.Contains(message, "missing accepted h2 heading"):
		return SourceClassificationAcceptedSectionMissing, "source is missing one or more accepted h2 sections"
	case strings.Contains(message, "duplicate accepted h2 heading"):
		return SourceClassificationAcceptedSectionDuplicate, "source contains a duplicate accepted h2 section"
	case strings.Contains(message, "ambiguous accepted h2 heading"):
		return SourceClassificationAcceptedSectionAmbiguous, "source contains an ambiguous accepted h2 section"
	case strings.Contains(message, "accepted requirements exceed"):
		return SourceClassificationRequirementsTooLarge, fmt.Sprintf("accepted requirements exceed %d bytes", maxAcceptedRequirementsBytes)
	default:
		return SourceClassificationInvalidChangeBody, "change source body is structurally invalid"
	}
}

func validInvalidSourceClassificationCode(code string) bool {
	switch code {
	case SourceClassificationObjectiveMarker,
		SourceClassificationUnsupportedMarker,
		SourceClassificationMultipleMarkers,
		SourceClassificationChangeMarkerRequired,
		SourceClassificationAcceptedSectionMissing,
		SourceClassificationAcceptedSectionDuplicate,
		SourceClassificationAcceptedSectionAmbiguous,
		SourceClassificationRequirementsTooLarge,
		SourceClassificationInvalidChangeBody:
		return true
	default:
		return false
	}
}

func cloneSourceParent(parent *SourceParent) *SourceParent {
	if parent == nil {
		return nil
	}
	copy := *parent
	return &copy
}

func clonePinnedSource(source *PinnedSource) *PinnedSource {
	if source == nil {
		return nil
	}
	copy := clonePinnedSourceValue(*source)
	return &copy
}

func clonePinnedSourceValue(source PinnedSource) PinnedSource {
	source.URLAliases = append([]string(nil), source.URLAliases...)
	if source.URLAliases == nil {
		source.URLAliases = make([]string, 0)
	}
	source.Parent = cloneSourceParent(source.Parent)
	return source
}

func cloneSourceCandidateInput(input SourceCandidateInput) SourceCandidateInput {
	input.URLAliases = append([]string(nil), input.URLAliases...)
	if input.URLAliases == nil {
		input.URLAliases = make([]string, 0)
	}
	input.Parent = cloneSourceParent(input.Parent)
	input.Snapshot = clonePinnedSource(input.Snapshot)
	return input
}

func validatePinnedSource(source PinnedSource) error {
	if err := validatePersistedSourceProjection(
		source.SourceVersion,
		source.ParserVersion,
		source.Provider,
		source.Host,
		source.RepositoryID,
		source.IssueID,
		source.IssueNumber,
		source.CanonicalURL,
		source.URLAliases,
		source.SourceRevision,
		source.Title,
		source.Parent,
	); err != nil {
		return err
	}
	if !validSHA256(source.RequirementsRevision) {
		return errors.New("requirements_revision must use lowercase sha256:<64 hex> format")
	}
	if err := validateAcceptedRequirements(source.AcceptedRequirements); err != nil {
		return err
	}
	total := len(source.AcceptedRequirements.OutcomeMarkdown) +
		len(source.AcceptedRequirements.RequirementsMarkdown) +
		len(source.AcceptedRequirements.AcceptanceExamplesMarkdown) +
		len(source.AcceptedRequirements.ConstraintsMarkdown) +
		len(source.AcceptedRequirements.NonGoalsMarkdown)
	if total > maxAcceptedRequirementsBytes {
		return fmt.Errorf("accepted requirements exceed %d bytes", maxAcceptedRequirementsBytes)
	}
	for name, value := range map[string]string{
		"accepted_requirements.outcome_markdown":             source.AcceptedRequirements.OutcomeMarkdown,
		"accepted_requirements.requirements_markdown":        source.AcceptedRequirements.RequirementsMarkdown,
		"accepted_requirements.acceptance_examples_markdown": source.AcceptedRequirements.AcceptanceExamplesMarkdown,
		"accepted_requirements.constraints_markdown":         source.AcceptedRequirements.ConstraintsMarkdown,
		"accepted_requirements.non_goals_markdown":           source.AcceptedRequirements.NonGoalsMarkdown,
	} {
		if err := validateC0Text(name, value, true); err != nil {
			return err
		}
	}
	if computed := requirementsRevision(source.AcceptedRequirements); source.RequirementsRevision != computed {
		return errors.New("requirements_revision does not match accepted_requirements")
	}
	return nil
}

func validateSourceCandidateInput(input SourceCandidateInput) error {
	if err := validatePersistedSourceProjection(
		input.SourceVersion,
		input.ParserVersion,
		input.Provider,
		input.Host,
		input.RepositoryID,
		input.IssueID,
		input.IssueNumber,
		input.CanonicalURL,
		input.URLAliases,
		input.SourceRevision,
		input.Title,
		input.Parent,
	); err != nil {
		return err
	}

	if input.Valid {
		if input.Classification != SourceClassificationValid || input.ClassificationCode != SourceClassificationValidChange || input.ClassificationError != "" {
			return errors.New("valid source candidate has inconsistent classification")
		}
		if input.Snapshot == nil {
			return errors.New("valid source candidate requires snapshot")
		}
		if err := validatePinnedSource(*input.Snapshot); err != nil {
			return fmt.Errorf("validate candidate snapshot: %w", err)
		}
		if input.RequirementsRevision == "" || !candidateMatchesSnapshot(input, *input.Snapshot) {
			return errors.New("valid source candidate does not match snapshot")
		}
		return nil
	}

	if input.Classification != SourceClassificationInvalid || !validInvalidSourceClassificationCode(input.ClassificationCode) {
		return errors.New("invalid source candidate has inconsistent classification")
	}
	if strings.TrimSpace(input.ClassificationError) == "" {
		return errors.New("invalid source candidate requires classification_error")
	}
	if len(input.ClassificationError) > maxSourceClassificationErrBytes {
		return fmt.Errorf("classification_error exceeds %d bytes", maxSourceClassificationErrBytes)
	}
	if err := validateC0Text("classification_error", input.ClassificationError, false); err != nil {
		return err
	}
	if input.RequirementsRevision != "" {
		return errors.New("invalid source candidate must omit requirements_revision")
	}
	if input.Snapshot != nil {
		return errors.New("invalid source candidate must omit snapshot")
	}
	return nil
}

func validatePersistedSourceProjection(sourceVersion, parserVersion int, provider, host, repositoryID, issueID string, issueNumber int, canonicalURL string, aliases []string, sourceRevision, title string, parent *SourceParent) error {
	if sourceVersion != SourceVersion {
		return fmt.Errorf("source_version must be %d", SourceVersion)
	}
	if parserVersion != ParserVersion {
		return fmt.Errorf("parser_version must be %d", ParserVersion)
	}
	if provider != "github" {
		return errors.New("provider must be exactly github")
	}
	if host != "github.com" {
		return errors.New("host must be exactly github.com")
	}
	if err := validateGitHubNodeID("repository_id", repositoryID); err != nil {
		return err
	}
	if err := validateGitHubNodeID("issue_id", issueID); err != nil {
		return err
	}
	if issueNumber <= 0 {
		return errors.New("issue_number must be positive")
	}
	if err := validateGitHubIssueURL("canonical_url", canonicalURL, issueNumber); err != nil {
		return err
	}
	if aliases == nil {
		return errors.New("url_aliases must be an initialized array")
	}
	seenAliases := make(map[string]struct{}, len(aliases))
	for index, alias := range aliases {
		field := fmt.Sprintf("url_aliases[%d]", index)
		if err := validateC0Text(field, alias, false); err != nil {
			return err
		}
		if err := validateGitHubIssueURL(field, alias, 0); err != nil {
			return err
		}
		if alias == canonicalURL {
			return fmt.Errorf("%s must differ from canonical_url", field)
		}
		if _, exists := seenAliases[alias]; exists {
			return fmt.Errorf("%s duplicates an earlier alias", field)
		}
		seenAliases[alias] = struct{}{}
	}
	if !validSHA256(sourceRevision) {
		return errors.New("source_revision must use lowercase sha256:<64 hex> format")
	}
	if err := validateC0Text("title", title, false); err != nil {
		return err
	}
	if strings.TrimSpace(title) == "" {
		return errors.New("title must be nonempty")
	}
	if parent != nil {
		if err := validateGitHubNodeID("parent.repository_id", parent.RepositoryID); err != nil {
			return err
		}
		if err := validateGitHubNodeID("parent.issue_id", parent.IssueID); err != nil {
			return err
		}
		if err := validateGitHubIssueURL("parent.canonical_url", parent.CanonicalURL, 0); err != nil {
			return err
		}
	}
	return nil
}

func candidateMatchesSnapshot(candidate SourceCandidateInput, snapshot PinnedSource) bool {
	return candidate.SourceVersion == snapshot.SourceVersion &&
		candidate.ParserVersion == snapshot.ParserVersion &&
		candidate.Provider == snapshot.Provider &&
		candidate.Host == snapshot.Host &&
		candidate.RepositoryID == snapshot.RepositoryID &&
		candidate.IssueID == snapshot.IssueID &&
		candidate.IssueNumber == snapshot.IssueNumber &&
		candidate.CanonicalURL == snapshot.CanonicalURL &&
		stringSlicesEqual(candidate.URLAliases, snapshot.URLAliases) &&
		candidate.SourceRevision == snapshot.SourceRevision &&
		candidate.RequirementsRevision == snapshot.RequirementsRevision &&
		candidate.Title == snapshot.Title &&
		sourceParentsEqual(candidate.Parent, snapshot.Parent)
}

func sourceParentsEqual(left, right *SourceParent) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func readSourceFile(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("read source file: path is empty")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("read source file: resolve path: %w", err)
	}

	root, err := os.OpenRoot(filepath.Dir(absolute))
	if err != nil {
		return nil, fmt.Errorf("read source file: open parent: %w", err)
	}
	raw, readErr := readSourceFileInRoot(root, filepath.Base(absolute))
	rootCloseErr := root.Close()
	if rootCloseErr != nil {
		readErr = errors.Join(readErr, fmt.Errorf("close source parent: %w", rootCloseErr))
	}
	if readErr != nil {
		return nil, fmt.Errorf("read source file: %w", readErr)
	}
	return raw, nil
}

func readSourceFileInRoot(root *os.Root, name string) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("inspect %q: %w", name, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("path %q is not a regular file", name)
	}

	file, err := root.OpenFile(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", name, err)
	}

	opened, statErr := file.Stat()
	current, lstatErr := root.Lstat(name)
	operationErr := sourceFileIdentityError(name, before, opened, current, statErr, lstatErr)
	var raw []byte
	if operationErr == nil {
		raw, operationErr = io.ReadAll(io.LimitReader(file, maxSourceFileBytes+1))
		if operationErr != nil {
			operationErr = fmt.Errorf("read %q: %w", name, operationErr)
		}
	}

	afterOpened, afterStatErr := file.Stat()
	afterCurrent, afterLstatErr := root.Lstat(name)
	if identityErr := sourceFileIdentityError(name, before, afterOpened, afterCurrent, afterStatErr, afterLstatErr); identityErr != nil {
		operationErr = errors.Join(operationErr, identityErr)
	}
	if closeErr := file.Close(); closeErr != nil {
		operationErr = errors.Join(operationErr, fmt.Errorf("close %q: %w", name, closeErr))
	}
	if operationErr != nil {
		return nil, operationErr
	}
	if len(raw) > maxSourceFileBytes {
		return nil, fmt.Errorf("source file exceeds %d bytes", maxSourceFileBytes)
	}
	return raw, nil
}

func sourceFileIdentityError(name string, before, opened, current os.FileInfo, statErr, lstatErr error) error {
	if statErr != nil {
		return fmt.Errorf("inspect opened %q: %w", name, statErr)
	}
	if lstatErr != nil {
		return fmt.Errorf("reinspect %q: %w", name, lstatErr)
	}
	if opened == nil || current == nil || !opened.Mode().IsRegular() || current.Mode()&os.ModeSymlink != 0 || !current.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, current) {
		return fmt.Errorf("path %q changed or is not the opened regular file", name)
	}
	return nil
}

func validateRawSourceEnvelope(envelope RawSourceEnvelope) error {
	if envelope.SourceVersion != SourceVersion {
		return fmt.Errorf("source_version must be %d", SourceVersion)
	}

	textFields := []struct {
		name  string
		value string
	}{
		{name: "provider", value: envelope.Provider},
		{name: "host", value: envelope.Host},
		{name: "repository_id", value: envelope.RepositoryID},
		{name: "issue_id", value: envelope.IssueID},
		{name: "canonical_url", value: envelope.CanonicalURL},
		{name: "updated_at", value: envelope.UpdatedAt},
		{name: "fetched_at", value: envelope.FetchedAt},
		{name: "title", value: envelope.Title},
	}
	for _, field := range textFields {
		if err := validateC0Text(field.name, field.value, false); err != nil {
			return err
		}
	}
	if err := validateC0Text("body", envelope.Body, true); err != nil {
		return err
	}

	if envelope.Provider != "github" {
		return errors.New("provider must be exactly github")
	}
	if envelope.Host != "github.com" {
		return errors.New("host must be exactly github.com")
	}
	if err := validateGitHubNodeID("repository_id", envelope.RepositoryID); err != nil {
		return err
	}
	if err := validateGitHubNodeID("issue_id", envelope.IssueID); err != nil {
		return err
	}
	if envelope.IssueNumber <= 0 {
		return errors.New("issue_number must be positive")
	}
	if strings.TrimSpace(envelope.Title) == "" {
		return errors.New("title must be nonempty")
	}
	if _, err := time.Parse(time.RFC3339, envelope.UpdatedAt); err != nil {
		return fmt.Errorf("updated_at must be rfc3339: %w", err)
	}
	if _, err := time.Parse(time.RFC3339, envelope.FetchedAt); err != nil {
		return fmt.Errorf("fetched_at must be rfc3339: %w", err)
	}
	if err := validateGitHubIssueURL("canonical_url", envelope.CanonicalURL, envelope.IssueNumber); err != nil {
		return err
	}
	if envelope.Labels == nil {
		return errors.New("labels must be an initialized array")
	}
	for index, label := range envelope.Labels {
		field := fmt.Sprintf("labels[%d]", index)
		if err := validateC0Text(field, label, false); err != nil {
			return err
		}
		if strings.TrimSpace(label) == "" {
			return fmt.Errorf("%s must be nonempty", field)
		}
	}

	if envelope.Parent != nil {
		parentFields := []struct {
			name  string
			value string
		}{
			{name: "parent.repository_id", value: envelope.Parent.RepositoryID},
			{name: "parent.issue_id", value: envelope.Parent.IssueID},
			{name: "parent.canonical_url", value: envelope.Parent.CanonicalURL},
		}
		for _, field := range parentFields {
			if err := validateC0Text(field.name, field.value, false); err != nil {
				return err
			}
		}
		if err := validateGitHubNodeID("parent.repository_id", envelope.Parent.RepositoryID); err != nil {
			return err
		}
		if err := validateGitHubNodeID("parent.issue_id", envelope.Parent.IssueID); err != nil {
			return err
		}
		if err := validateGitHubIssueURL("parent.canonical_url", envelope.Parent.CanonicalURL, 0); err != nil {
			return err
		}
	}
	return nil
}

func validateC0Text(field, value string, markdownBody bool) error {
	for _, character := range value {
		if character >= 0x20 {
			continue
		}
		if markdownBody && (character == '\t' || character == '\n' || character == '\r') {
			continue
		}
		return fmt.Errorf("%s contains disallowed c0 control u+%04x", field, character)
	}
	return nil
}

func validateGitHubNodeID(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s must be a nonempty github node id", field)
	}
	for _, character := range value {
		if unicode.IsSpace(character) {
			return fmt.Errorf("%s must be a github node id without whitespace", field)
		}
	}
	return nil
}

func validateGitHubIssueURL(field, value string, expectedNumber int) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a github issue url: %w", field, err)
	}
	if parsed.Scheme != "https" || parsed.Opaque != "" || parsed.User != nil || parsed.Hostname() != "github.com" {
		return fmt.Errorf("%s must use https://github.com without userinfo", field)
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return fmt.Errorf("%s must not use a non-default port", field)
	}
	if parsed.Host != "github.com" && parsed.Host != "github.com:443" {
		return fmt.Errorf("%s must use the exact github.com host", field)
	}
	if parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(value, "#") {
		return fmt.Errorf("%s must not contain a query or fragment", field)
	}
	if parsed.RawPath != "" {
		return fmt.Errorf("%s must not contain escaped path segments", field)
	}

	segments := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
	if !strings.HasPrefix(parsed.Path, "/") || len(segments) != 4 || segments[0] == "" || segments[1] == "" || segments[2] != "issues" {
		return fmt.Errorf("%s must match https://github.com/<owner>/<repo>/issues/<number>", field)
	}
	if !validGitHubOwner(segments[0]) || !validGitHubRepositoryName(segments[1]) {
		return fmt.Errorf("%s contains an invalid github owner or repository", field)
	}
	number, err := strconv.ParseUint(segments[3], 10, 64)
	if err != nil || number == 0 || strconv.FormatUint(number, 10) != segments[3] {
		return fmt.Errorf("%s issue number must be a positive canonical decimal", field)
	}
	if expectedNumber > 0 && number != uint64(expectedNumber) {
		return fmt.Errorf("%s issue number must match issue_number", field)
	}
	return nil
}

func validGitHubOwner(value string) bool {
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return value != ""
}

func validGitHubRepositoryName(value string) bool {
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return value != "" && value != "." && value != ".."
}

func normalizeLineEndings(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

type markdownLine struct {
	text         string
	start        int
	contentStart int
}

type acceptedSectionLocation struct {
	start        int
	contentStart int
}

func parseAcceptedRequirements(body string) (AcceptedRequirements, error) {
	lines := splitMarkdownLines(body)
	firstNonempty := ""
	for _, line := range lines {
		if strings.TrimSpace(line.text) != "" {
			firstNonempty = line.text
			break
		}
	}
	if firstNonempty != changeSourceMarker {
		if firstNonempty == "<!-- slipway-level: objective/v1 -->" {
			return AcceptedRequirements{}, errors.New("objective marker cannot be used as a change source")
		}
		if strings.Contains(firstNonempty, markerIdentifier) {
			return AcceptedRequirements{}, errors.New("unsupported slipway-level marker")
		}
		return AcceptedRequirements{}, fmt.Errorf("first nonempty body line must exactly equal %s", changeSourceMarker)
	}

	headingIndexes := make(map[string]int, len(acceptedSectionHeadings))
	for index, heading := range acceptedSectionHeadings {
		headingIndexes[heading] = index
	}
	locations := make([]*acceptedSectionLocation, len(acceptedSectionHeadings))
	allH2Starts := make([]int, 0, len(acceptedSectionHeadings)+1)
	markerCount := 0
	var fenceCharacter byte
	fenceLength := 0

	for _, line := range lines {
		if fenceLength > 0 {
			if closesFence(line.text, fenceCharacter, fenceLength) {
				fenceCharacter = 0
				fenceLength = 0
			}
			continue
		}
		if character, length, ok := opensFence(line.text); ok {
			fenceCharacter = character
			fenceLength = length
			continue
		}

		markerCount += strings.Count(line.text, markerIdentifier)
		heading, isH2 := markdownH2Text(line.text)
		if !isH2 {
			if acceptedHeadingLookalike(line.text) {
				return AcceptedRequirements{}, fmt.Errorf("ambiguous accepted h2 heading %q", line.text)
			}
			continue
		}
		allH2Starts = append(allH2Starts, line.start)

		index, accepted := headingIndexes[heading]
		if !accepted {
			if acceptedHeadingNameFold(heading) {
				return AcceptedRequirements{}, fmt.Errorf("ambiguous accepted h2 heading %q", line.text)
			}
			continue
		}
		if line.text != "## "+heading {
			return AcceptedRequirements{}, fmt.Errorf("ambiguous accepted h2 heading %q", line.text)
		}
		if locations[index] != nil {
			return AcceptedRequirements{}, fmt.Errorf("duplicate accepted h2 heading %q", heading)
		}
		locations[index] = &acceptedSectionLocation{start: line.start, contentStart: line.contentStart}
	}

	if markerCount != 1 {
		if markerCount == 0 {
			return AcceptedRequirements{}, errors.New("change source marker must appear outside code fences")
		}
		return AcceptedRequirements{}, errors.New("multiple slipway-level markers outside code fences")
	}

	sections := make([]string, len(locations))
	totalBytes := 0
	for index, location := range locations {
		if location == nil {
			return AcceptedRequirements{}, fmt.Errorf("missing accepted h2 heading %q", acceptedSectionHeadings[index])
		}
		end := len(body)
		for _, headingStart := range allH2Starts {
			if headingStart > location.start {
				end = headingStart
				break
			}
		}
		sections[index] = body[location.contentStart:end]
		totalBytes += len(sections[index])
	}
	if totalBytes > maxAcceptedRequirementsBytes {
		return AcceptedRequirements{}, fmt.Errorf("accepted requirements exceed %d bytes", maxAcceptedRequirementsBytes)
	}

	return AcceptedRequirements{
		OutcomeMarkdown:            sections[0],
		RequirementsMarkdown:       sections[1],
		AcceptanceExamplesMarkdown: sections[2],
		ConstraintsMarkdown:        sections[3],
		NonGoalsMarkdown:           sections[4],
	}, nil
}

func splitMarkdownLines(body string) []markdownLine {
	lines := make([]markdownLine, 0, strings.Count(body, "\n")+1)
	for start := 0; start < len(body); {
		relativeEnd := strings.IndexByte(body[start:], '\n')
		if relativeEnd < 0 {
			lines = append(lines, markdownLine{text: body[start:], start: start, contentStart: len(body)})
			break
		}
		end := start + relativeEnd
		lines = append(lines, markdownLine{text: body[start:end], start: start, contentStart: end + 1})
		start = end + 1
	}
	return lines
}

func opensFence(line string) (byte, int, bool) {
	trimmed, ok := trimFenceIndent(line)
	if !ok || trimmed == "" || (trimmed[0] != '`' && trimmed[0] != '~') {
		return 0, 0, false
	}
	character := trimmed[0]
	length := countLeadingByte(trimmed, character)
	if length < 3 {
		return 0, 0, false
	}
	if character == '`' && strings.Contains(trimmed[length:], "`") {
		return 0, 0, false
	}
	return character, length, true
}

func closesFence(line string, character byte, minimumLength int) bool {
	trimmed, ok := trimFenceIndent(line)
	if !ok || trimmed == "" || trimmed[0] != character {
		return false
	}
	length := countLeadingByte(trimmed, character)
	return length >= minimumLength && strings.Trim(trimmed[length:], " \t") == ""
}

func trimFenceIndent(line string) (string, bool) {
	spaces := 0
	for spaces < len(line) && line[spaces] == ' ' {
		spaces++
	}
	if spaces > 3 {
		return "", false
	}
	return line[spaces:], true
}

func countLeadingByte(value string, character byte) int {
	count := 0
	for count < len(value) && value[count] == character {
		count++
	}
	return count
}

func markdownH2Text(line string) (string, bool) {
	trimmed, ok := trimFenceIndent(line)
	if !ok || !strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "###") {
		return "", false
	}
	if len(trimmed) == 2 {
		return "", true
	}
	if trimmed[2] != ' ' && trimmed[2] != '\t' {
		return "", false
	}
	content := strings.Trim(trimmed[2:], " \t")
	if closingStart := closingHeadingHashStart(content); closingStart >= 0 {
		content = strings.TrimRight(content[:closingStart], " \t")
	}
	return content, true
}

func closingHeadingHashStart(content string) int {
	index := len(content)
	for index > 0 && content[index-1] == '#' {
		index--
	}
	if index == len(content) || index == 0 || (content[index-1] != ' ' && content[index-1] != '\t') {
		return -1
	}
	return index
}

func acceptedHeadingLookalike(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "###") {
		return false
	}
	candidate := strings.TrimSpace(strings.TrimPrefix(trimmed, "##"))
	candidate = strings.TrimSpace(strings.TrimRight(candidate, "#"))
	return acceptedHeadingNameFold(candidate)
}

func acceptedHeadingNameFold(value string) bool {
	for _, heading := range acceptedSectionHeadings {
		if strings.EqualFold(value, heading) {
			return true
		}
	}
	return false
}

func sourceRevision(envelope RawSourceEnvelope, normalizedBody string) string {
	return framedRevision(
		strconv.Itoa(SourceVersion),
		envelope.Host,
		envelope.RepositoryID,
		envelope.IssueID,
		normalizeLineEndings(envelope.Title),
		normalizedBody,
	)
}

func requirementsRevision(requirements AcceptedRequirements) string {
	return framedRevision(
		strconv.Itoa(ParserVersion),
		requirements.OutcomeMarkdown,
		requirements.RequirementsMarkdown,
		requirements.AcceptanceExamplesMarkdown,
		requirements.ConstraintsMarkdown,
		requirements.NonGoalsMarkdown,
	)
}

// framedRevision hashes each ordered UTF-8 field as an unsigned uint64
// big-endian byte length followed immediately by the exact field bytes. There
// is no field count, delimiter, terminator, or Unicode normalization.
func framedRevision(fields ...string) string {
	hasher := sha256.New()
	var lengthPrefix [8]byte
	for _, field := range fields {
		binary.BigEndian.PutUint64(lengthPrefix[:], uint64(len(field)))
		_, _ = hasher.Write(lengthPrefix[:])
		_, _ = io.WriteString(hasher, field)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}
