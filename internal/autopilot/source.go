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
	SourceVersion = 2
	ParserVersion = 2

	maxSourceFileBytes              = 16 << 20
	maxSourceManifestBytes          = 256 << 10
	maxSourceSections               = 64
	maxSourceLabels                 = 100
	maxSourceURLAliases             = 64
	maxSourceSectionBytes           = 256 << 10
	maxSourceMaterialBytes          = 4 << 20
	maxSourceClassificationErrBytes = 1 << 10
)

const (
	SourceManifestVersion = 2
	SourceProfileChangeV2 = "change/v2"

	changeSourceMarker  = "<!-- slipway-level: change/v2 -->"
	sourceManifestFence = "```slipway-manifest"
	sectionMarkerPrefix = "<!-- slipway-section:v1 key="
)

// SourceParent identifies an optional parent Objective for traceability only.
type SourceParent struct {
	RepositoryID string `json:"repository_id"`
	IssueID      string `json:"issue_id"`
	CanonicalURL string `json:"canonical_url"`
}

// RawSourceEnvelope is the strict, ephemeral GitHub input supplied by a host.
// For a valid manifest, Comments contains exactly its referenced comments. An
// invalid head uses an initialized empty slice; discussion never enters here.
type RawSourceEnvelope struct {
	SourceVersion int                `json:"source_version"`
	Provider      string             `json:"provider"`
	Host          string             `json:"host"`
	RepositoryID  string             `json:"repository_id"`
	IssueID       string             `json:"issue_id"`
	IssueNumber   int                `json:"issue_number"`
	CanonicalURL  string             `json:"canonical_url"`
	UpdatedAt     string             `json:"updated_at"`
	FetchedAt     string             `json:"fetched_at"`
	Title         string             `json:"title"`
	Body          string             `json:"body"`
	Labels        []string           `json:"labels"`
	Parent        *SourceParent      `json:"parent,omitempty"`
	Comments      []RawSourceComment `json:"comments"`
}

// RawSourceComment is one manifest-referenced GitHub Issue comment observation.
type RawSourceComment struct {
	NodeID      string `json:"node_id"`
	DatabaseID  int64  `json:"database_id"`
	URL         string `json:"url"`
	UpdatedAt   string `json:"updated_at"`
	AuthorID    string `json:"author_id"`
	IsMinimized bool   `json:"is_minimized"`
	Body        string `json:"body"`
}

// SourceSectionRole identifies how a normative chapter contributes to a Change.
type SourceSectionRole string

const (
	SourceSectionOutcome            SourceSectionRole = "outcome"
	SourceSectionRequirements       SourceSectionRole = "requirements"
	SourceSectionAcceptanceExamples SourceSectionRole = "acceptance_examples"
	SourceSectionConstraints        SourceSectionRole = "constraints"
	SourceSectionNonGoals           SourceSectionRole = "non_goals"
)

// SourceManifest is the only accepted Issue source head. Array order is
// normative; comment order and timestamps are not.
type SourceManifest struct {
	ManifestVersion            int                     `json:"manifest_version"`
	Profile                    string                  `json:"profile"`
	ParentRequirementsRevision string                  `json:"parent_requirements_revision,omitempty"`
	Sections                   []SourceManifestSection `json:"sections"`
}

// SourceManifestSection binds one stable key and role to an exact comment body.
type SourceManifestSection struct {
	Key               string            `json:"key"`
	Role              SourceSectionRole `json:"role"`
	Title             string            `json:"title"`
	CommentNodeID     string            `json:"comment_node_id"`
	CommentDatabaseID int64             `json:"comment_database_id"`
	BodySHA256        string            `json:"body_sha256"`
}

// SourceSectionProvenance records non-authoritative fetch and display metadata.
type SourceSectionProvenance struct {
	CommentNodeID     string `json:"comment_node_id"`
	CommentDatabaseID int64  `json:"comment_database_id"`
	URL               string `json:"url"`
	AuthorID          string `json:"author_id"`
	ObservedUpdatedAt string `json:"observed_updated_at"`
}

// PinnedSourceSection is the persisted, path-free catalog entry for one local
// content-addressed chapter. Markdown is stored separately by runstore.
type PinnedSourceSection struct {
	Key             string                  `json:"key"`
	Role            SourceSectionRole       `json:"role"`
	Title           string                  `json:"title"`
	BodySHA256      string                  `json:"body_sha256"`
	SectionRevision string                  `json:"section_revision"`
	MaterialSHA256  string                  `json:"material_sha256"`
	Bytes           int                     `json:"bytes"`
	Provenance      SourceSectionProvenance `json:"provenance"`
}

type sourceMaterial struct {
	Digest string
	Data   []byte
}

// PinnedSource is the complete source catalog that may be persisted. Raw Issue
// bodies, labels, timestamps, and Markdown material are not journaled.
type PinnedSource struct {
	SourceVersion              int                   `json:"source_version"`
	ParserVersion              int                   `json:"parser_version"`
	ManifestVersion            int                   `json:"manifest_version"`
	Profile                    string                `json:"profile"`
	Provider                   string                `json:"provider"`
	Host                       string                `json:"host"`
	RepositoryID               string                `json:"repository_id"`
	IssueID                    string                `json:"issue_id"`
	IssueNumber                int                   `json:"issue_number"`
	CanonicalURL               string                `json:"canonical_url"`
	URLAliases                 []string              `json:"url_aliases"`
	SourceRevision             string                `json:"source_revision"`
	ManifestRevision           string                `json:"manifest_revision"`
	RequirementsRevision       string                `json:"requirements_revision"`
	ParentRequirementsRevision string                `json:"parent_requirements_revision,omitempty"`
	Title                      string                `json:"title"`
	Parent                     *SourceParent         `json:"parent,omitempty"`
	Sections                   []PinnedSourceSection `json:"sections"`
	materials                  []sourceMaterial
}

// SourceClassification reports whether a refreshed envelope is a structurally
// valid Change. Identity and projection failures are errors, not classifications.
type SourceClassification string

const (
	SourceClassificationValid   SourceClassification = "valid"
	SourceClassificationInvalid SourceClassification = "invalid"
)

const (
	SourceClassificationValidChange          = "valid_change"
	SourceClassificationObjectiveMarker      = "objective_marker"
	SourceClassificationUnsupportedMarker    = "unsupported_marker"
	SourceClassificationChangeMarkerRequired = "change_marker_required"
	SourceClassificationManifestInvalid      = "source_manifest_invalid"
	SourceClassificationSectionMissing       = "source_section_missing"
	SourceClassificationSectionUnexpected    = "source_section_unexpected"
	SourceClassificationSectionMinimized     = "source_section_minimized"
	SourceClassificationSectionHashMismatch  = "source_section_hash_mismatch"
	SourceClassificationSectionInvalid       = "source_section_invalid"
	SourceClassificationSectionTooLarge      = "source_section_too_large"
	SourceClassificationBundleTooLarge       = "source_bundle_too_large"
	SourceClassificationInvalidChangeBody    = "invalid_change_body"
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
	manifest, sections, materials, bodyErr := parseSourceBundle(envelope, normalizedBody)
	if bodyErr != nil {
		candidate.Classification = SourceClassificationInvalid
		candidate.ClassificationCode, candidate.ClassificationError = classifySourceBodyError(bodyErr)
		return candidate, bodyErr, nil
	}

	manifestSHA256 := manifestRevision(manifest)
	snapshot := PinnedSource{
		SourceVersion:              SourceVersion,
		ParserVersion:              ParserVersion,
		ManifestVersion:            manifest.ManifestVersion,
		Profile:                    manifest.Profile,
		Provider:                   envelope.Provider,
		Host:                       envelope.Host,
		RepositoryID:               envelope.RepositoryID,
		IssueID:                    envelope.IssueID,
		IssueNumber:                envelope.IssueNumber,
		CanonicalURL:               envelope.CanonicalURL,
		URLAliases:                 make([]string, 0),
		SourceRevision:             sourceRevision(envelope, manifestSHA256),
		ManifestRevision:           manifestSHA256,
		RequirementsRevision:       requirementsRevision(manifest.Profile, sections),
		ParentRequirementsRevision: manifest.ParentRequirementsRevision,
		Title:                      envelope.Title,
		Parent:                     cloneSourceParent(envelope.Parent),
		Sections:                   sections,
		materials:                  cloneSourceMaterials(materials),
	}
	candidate.SourceRevision = snapshot.SourceRevision
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
		SourceRevision: observedSourceRevision(envelope, normalizedBody),
		Title:          envelope.Title,
		Parent:         cloneSourceParent(envelope.Parent),
	}
}

func classifySourceBodyError(err error) (string, string) {
	var bundleErr *sourceBundleError
	if errors.As(err, &bundleErr) {
		return bundleErr.code, bundleErr.message
	}
	return SourceClassificationInvalidChangeBody, "change source bundle is structurally invalid"
}

func validInvalidSourceClassificationCode(code string) bool {
	switch code {
	case SourceClassificationObjectiveMarker,
		SourceClassificationUnsupportedMarker,
		SourceClassificationChangeMarkerRequired,
		SourceClassificationManifestInvalid,
		SourceClassificationSectionMissing,
		SourceClassificationSectionUnexpected,
		SourceClassificationSectionMinimized,
		SourceClassificationSectionHashMismatch,
		SourceClassificationSectionInvalid,
		SourceClassificationSectionTooLarge,
		SourceClassificationBundleTooLarge,
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
	source.Sections = append([]PinnedSourceSection(nil), source.Sections...)
	if source.Sections == nil {
		source.Sections = make([]PinnedSourceSection, 0)
	}
	source.materials = cloneSourceMaterials(source.materials)
	return source
}

func cloneSourceMaterials(materials []sourceMaterial) []sourceMaterial {
	if materials == nil {
		return nil
	}
	cloned := make([]sourceMaterial, len(materials))
	for index, material := range materials {
		cloned[index] = sourceMaterial{
			Digest: material.Digest,
			Data:   append([]byte(nil), material.Data...),
		}
	}
	return cloned
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
	if source.ManifestVersion != SourceManifestVersion {
		return fmt.Errorf("manifest_version must be %d", SourceManifestVersion)
	}
	if source.Profile != SourceProfileChangeV2 {
		return fmt.Errorf("profile must be exactly %s", SourceProfileChangeV2)
	}
	if !validSHA256(source.ManifestRevision) {
		return errors.New("manifest_revision must use lowercase sha256:<64 hex> format")
	}
	if !validSHA256(source.RequirementsRevision) {
		return errors.New("requirements_revision must use lowercase sha256:<64 hex> format")
	}
	if source.ParentRequirementsRevision != "" && !validSHA256(source.ParentRequirementsRevision) {
		return errors.New("parent_requirements_revision must use lowercase sha256:<64 hex> format")
	}
	if err := validatePinnedSections(source.Sections); err != nil {
		return err
	}
	if err := validateSourceMaterials(source, false); err != nil {
		return err
	}
	manifest := manifestFromPinnedSource(source)
	if err := validateSourceManifest(manifest); err != nil {
		return fmt.Errorf("validate source manifest: %w", err)
	}
	for index, section := range source.Sections {
		if err := validatePinnedCommentURL(
			fmt.Sprintf("sections[%d].provenance.url", index),
			section.Provenance.URL,
			source.CanonicalURL,
			source.URLAliases,
			section.Provenance.CommentDatabaseID,
		); err != nil {
			return err
		}
	}
	if computed := manifestRevision(manifest); source.ManifestRevision != computed {
		return errors.New("manifest_revision does not match sections")
	}
	if computed := sourceRevisionFromIdentity(
		source.Host,
		source.RepositoryID,
		source.IssueID,
		source.Title,
		source.ManifestRevision,
	); source.SourceRevision != computed {
		return errors.New("source_revision does not match source identity and manifest")
	}
	if computed := requirementsRevision(source.Profile, source.Sections); source.RequirementsRevision != computed {
		return errors.New("requirements_revision does not match sections")
	}
	return nil
}

func validatePinnedCommentURL(field, value, canonicalURL string, aliases []string, databaseID int64) error {
	canonicalErr := validateGitHubCommentURL(field, value, canonicalURL, databaseID)
	if canonicalErr == nil {
		return nil
	}
	for _, alias := range aliases {
		if validateGitHubCommentURL(field, value, alias, databaseID) == nil {
			return nil
		}
	}
	return canonicalErr
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
	if err := validateTextControls("classification_error", input.ClassificationError, false); err != nil {
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
	if len(aliases) > maxSourceURLAliases {
		return fmt.Errorf("url_aliases must contain at most %d entries", maxSourceURLAliases)
	}
	seenAliases := make(map[string]struct{}, len(aliases))
	for index, alias := range aliases {
		field := fmt.Sprintf("url_aliases[%d]", index)
		if err := validateTextControls(field, alias, false); err != nil {
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
	if err := validateTextControls("title", title, false); err != nil {
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
		if err := validateTextControls(field.name, field.value, false); err != nil {
			return err
		}
	}
	if err := validateTextControls("body", envelope.Body, true); err != nil {
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
	if len(envelope.Labels) > maxSourceLabels {
		return fmt.Errorf("labels must contain at most %d entries", maxSourceLabels)
	}
	for index, label := range envelope.Labels {
		field := fmt.Sprintf("labels[%d]", index)
		if err := validateTextControls(field, label, false); err != nil {
			return err
		}
		if strings.TrimSpace(label) == "" {
			return fmt.Errorf("%s must be nonempty", field)
		}
	}
	if envelope.Comments == nil {
		return errors.New("comments must be an initialized array")
	}
	if len(envelope.Comments) > maxSourceSections {
		return fmt.Errorf("comments must contain at most %d entries", maxSourceSections)
	}
	nodeIDs := make(map[string]struct{}, len(envelope.Comments))
	databaseIDs := make(map[int64]struct{}, len(envelope.Comments))
	for index, comment := range envelope.Comments {
		field := fmt.Sprintf("comments[%d]", index)
		if err := validateRawSourceComment(field, comment, envelope.CanonicalURL); err != nil {
			return err
		}
		if _, exists := nodeIDs[comment.NodeID]; exists {
			return fmt.Errorf("%s.node_id is duplicated", field)
		}
		nodeIDs[comment.NodeID] = struct{}{}
		if _, exists := databaseIDs[comment.DatabaseID]; exists {
			return fmt.Errorf("%s.database_id is duplicated", field)
		}
		databaseIDs[comment.DatabaseID] = struct{}{}
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
			if err := validateTextControls(field.name, field.value, false); err != nil {
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

func validateTextControls(field, value string, markdownBody bool) error {
	for _, character := range value {
		if !unicode.IsControl(character) {
			continue
		}
		if markdownBody && (character == '\t' || character == '\n' || character == '\r') {
			continue
		}
		return fmt.Errorf("%s contains disallowed control u+%04x", field, character)
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
	if parsed.Port() != "" || parsed.Host != "github.com" {
		return fmt.Errorf("%s must use the exact github.com host without an explicit port", field)
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
