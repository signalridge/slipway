package autopilot

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const maxSourceSectionTitleBytes = 512

type sourceBundleError struct {
	code    string
	message string
}

func (e *sourceBundleError) Error() string {
	return e.message
}

func newSourceBundleError(code, message string) error {
	return &sourceBundleError{code: code, message: message}
}

func parseSourceBundle(
	envelope RawSourceEnvelope,
	normalizedIssueBody string,
) (SourceManifest, []PinnedSourceSection, []sourceMaterial, error) {
	manifest, err := parseSourceManifest(normalizedIssueBody)
	if err != nil {
		return SourceManifest{}, nil, nil, err
	}

	comments := make(map[string]RawSourceComment, len(envelope.Comments))
	for _, comment := range envelope.Comments {
		comments[comment.NodeID] = comment
	}
	sections := make([]PinnedSourceSection, 0, len(manifest.Sections))
	materials := make([]sourceMaterial, 0, len(manifest.Sections))
	totalBytes := 0
	for _, reference := range manifest.Sections {
		comment, ok := comments[reference.CommentNodeID]
		if !ok {
			return SourceManifest{}, nil, nil, newSourceBundleError(
				SourceClassificationSectionMissing,
				fmt.Sprintf("manifest section %q is missing its referenced comment", reference.Key),
			)
		}
		delete(comments, reference.CommentNodeID)
		if comment.DatabaseID != reference.CommentDatabaseID {
			return SourceManifest{}, nil, nil, newSourceBundleError(
				SourceClassificationSectionInvalid,
				fmt.Sprintf("manifest section %q comment database id does not match", reference.Key),
			)
		}
		if comment.IsMinimized {
			return SourceManifest{}, nil, nil, newSourceBundleError(
				SourceClassificationSectionMinimized,
				fmt.Sprintf("manifest section %q references a minimized comment", reference.Key),
			)
		}

		normalizedComment := normalizeLineEndings(comment.Body)
		bodySHA256 := commentBodyRevision(normalizedComment)
		if bodySHA256 != reference.BodySHA256 {
			return SourceManifest{}, nil, nil, newSourceBundleError(
				SourceClassificationSectionHashMismatch,
				fmt.Sprintf("manifest section %q comment body digest does not match", reference.Key),
			)
		}
		payload, parseErr := parseSourceSection(reference.Key, normalizedComment)
		if parseErr != nil {
			return SourceManifest{}, nil, nil, parseErr
		}
		if len(payload) > maxSourceSectionBytes {
			return SourceManifest{}, nil, nil, newSourceBundleError(
				SourceClassificationSectionTooLarge,
				fmt.Sprintf("manifest section %q exceeds %d bytes", reference.Key, maxSourceSectionBytes),
			)
		}
		if totalBytes > maxSourceMaterialBytes-len(payload) {
			return SourceManifest{}, nil, nil, newSourceBundleError(
				SourceClassificationBundleTooLarge,
				fmt.Sprintf("source bundle exceeds %d bytes", maxSourceMaterialBytes),
			)
		}
		totalBytes += len(payload)

		materialSHA256 := materialRevision(payload)
		section := PinnedSourceSection{
			Key:             reference.Key,
			Role:            reference.Role,
			Title:           reference.Title,
			BodySHA256:      bodySHA256,
			SectionRevision: sectionRevision(reference.Key, reference.Role, reference.Title, payload),
			MaterialSHA256:  materialSHA256,
			Bytes:           len(payload),
			Provenance: SourceSectionProvenance{
				CommentNodeID:     comment.NodeID,
				CommentDatabaseID: comment.DatabaseID,
				URL:               comment.URL,
				AuthorID:          comment.AuthorID,
				ObservedUpdatedAt: comment.UpdatedAt,
			},
		}
		sections = append(sections, section)
		materials = append(materials, sourceMaterial{
			Digest: materialSHA256,
			Data:   append([]byte(nil), []byte(payload)...),
		})
	}
	if len(comments) != 0 {
		return SourceManifest{}, nil, nil, newSourceBundleError(
			SourceClassificationSectionUnexpected,
			"source envelope contains a comment not referenced by the manifest",
		)
	}
	return manifest, sections, materials, nil
}

func parseSourceManifest(body string) (SourceManifest, error) {
	lines := strings.Split(body, "\n")
	markerIndex := firstNonemptyLine(lines, 0)
	if markerIndex < 0 {
		return SourceManifest{}, newSourceBundleError(
			SourceClassificationChangeMarkerRequired,
			fmt.Sprintf("source must begin with %s", changeSourceMarker),
		)
	}
	if lines[markerIndex] != changeSourceMarker {
		code := SourceClassificationChangeMarkerRequired
		if strings.Contains(lines[markerIndex], "slipway-level: objective/") {
			code = SourceClassificationObjectiveMarker
		} else if strings.Contains(lines[markerIndex], "slipway-level:") {
			code = SourceClassificationUnsupportedMarker
		}
		return SourceManifest{}, newSourceBundleError(code, fmt.Sprintf("source must begin with %s", changeSourceMarker))
	}

	fenceIndex := firstNonemptyLine(lines, markerIndex+1)
	if fenceIndex < 0 || lines[fenceIndex] != sourceManifestFence {
		return SourceManifest{}, newSourceBundleError(
			SourceClassificationManifestInvalid,
			fmt.Sprintf("source marker must be followed by %s", sourceManifestFence),
		)
	}
	closingIndex := -1
	for index := fenceIndex + 1; index < len(lines); index++ {
		if lines[index] == "```" {
			closingIndex = index
			break
		}
	}
	if closingIndex < 0 {
		return SourceManifest{}, newSourceBundleError(
			SourceClassificationManifestInvalid,
			"source manifest fence is not closed",
		)
	}
	var fenceByte byte
	fenceLength := 0
	for index := closingIndex + 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		candidateByte, candidateLength, candidateFence := markdownFenceDelimiter(lines[index])
		if fenceLength == 0 {
			if trimmed == sourceManifestFence {
				return SourceManifest{}, newSourceBundleError(
					SourceClassificationManifestInvalid,
					"source body contains multiple manifest fences",
				)
			}
			if candidateFence {
				fenceByte = candidateByte
				fenceLength = candidateLength
				continue
			}
			if strings.Contains(lines[index], "<!-- slipway-level:") {
				return SourceManifest{}, newSourceBundleError(
					SourceClassificationManifestInvalid,
					"source body contains an additional slipway-level marker outside a code fence",
				)
			}
			continue
		}
		if candidateFence && candidateByte == fenceByte && candidateLength >= fenceLength && markdownFenceCloses(lines[index], candidateByte, candidateLength) {
			fenceByte = 0
			fenceLength = 0
		}
	}

	manifestJSON := strings.Join(lines[fenceIndex+1:closingIndex], "\n")
	if len(manifestJSON) == 0 {
		return SourceManifest{}, newSourceBundleError(
			SourceClassificationManifestInvalid,
			"source manifest is empty",
		)
	}
	if len(manifestJSON) > maxSourceManifestBytes {
		return SourceManifest{}, newSourceBundleError(
			SourceClassificationManifestInvalid,
			fmt.Sprintf("source manifest exceeds %d bytes", maxSourceManifestBytes),
		)
	}
	var manifest SourceManifest
	if err := decodeStrictJSON([]byte(manifestJSON), &manifest); err != nil {
		return SourceManifest{}, newSourceBundleError(
			SourceClassificationManifestInvalid,
			"source manifest is invalid json: "+err.Error(),
		)
	}
	if err := validateSourceManifest(manifest); err != nil {
		return SourceManifest{}, newSourceBundleError(
			SourceClassificationManifestInvalid,
			"source manifest is invalid: "+err.Error(),
		)
	}
	return manifest, nil
}

func markdownFenceDelimiter(line string) (byte, int, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || len(trimmed) < 3 {
		return 0, 0, false
	}
	marker := trimmed[0]
	if marker != '`' && marker != '~' {
		return 0, 0, false
	}
	length := 0
	for length < len(trimmed) && trimmed[length] == marker {
		length++
	}
	return marker, length, length >= 3
}

func markdownFenceCloses(line string, marker byte, length int) bool {
	trimmed := strings.TrimLeft(line, " ")
	return len(trimmed) >= length && strings.TrimSpace(trimmed[length:]) == "" && trimmed[0] == marker
}

func firstNonemptyLine(lines []string, start int) int {
	for index := start; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) != "" {
			return index
		}
	}
	return -1
}

func validateSourceManifest(manifest SourceManifest) error {
	if manifest.ManifestVersion != SourceManifestVersion {
		return fmt.Errorf("manifest_version must be %d", SourceManifestVersion)
	}
	if manifest.Profile != SourceProfileChangeV2 {
		return fmt.Errorf("profile must be exactly %s", SourceProfileChangeV2)
	}
	if manifest.ParentRequirementsRevision != "" && !validSHA256(manifest.ParentRequirementsRevision) {
		return errors.New("parent_requirements_revision must use lowercase sha256:<64 hex> format")
	}
	if manifest.Sections == nil {
		return errors.New("sections must be an initialized array")
	}
	if len(manifest.Sections) < 5 || len(manifest.Sections) > maxSourceSections {
		return fmt.Errorf("sections must contain 5..%d entries", maxSourceSections)
	}

	keys := make(map[string]struct{}, len(manifest.Sections))
	nodeIDs := make(map[string]struct{}, len(manifest.Sections))
	databaseIDs := make(map[int64]struct{}, len(manifest.Sections))
	roles := make(map[SourceSectionRole]int, 5)
	for index, section := range manifest.Sections {
		field := fmt.Sprintf("sections[%d]", index)
		if !validSourceSectionKey(section.Key) {
			return fmt.Errorf("%s.key must be 1..64 lowercase ascii key characters", field)
		}
		if _, exists := keys[section.Key]; exists {
			return fmt.Errorf("%s.key duplicates %q", field, section.Key)
		}
		keys[section.Key] = struct{}{}
		if !validSourceSectionRole(section.Role) {
			return fmt.Errorf("%s.role %q is unsupported", field, section.Role)
		}
		roles[section.Role]++
		if err := validateTextControls(field+".title", section.Title, false); err != nil {
			return err
		}
		if strings.TrimSpace(section.Title) == "" || len(section.Title) > maxSourceSectionTitleBytes {
			return fmt.Errorf("%s.title must contain 1..%d bytes", field, maxSourceSectionTitleBytes)
		}
		if err := validateGitHubNodeID(field+".comment_node_id", section.CommentNodeID); err != nil {
			return err
		}
		if _, exists := nodeIDs[section.CommentNodeID]; exists {
			return fmt.Errorf("%s.comment_node_id is duplicated", field)
		}
		nodeIDs[section.CommentNodeID] = struct{}{}
		if section.CommentDatabaseID <= 0 {
			return fmt.Errorf("%s.comment_database_id must be positive", field)
		}
		if _, exists := databaseIDs[section.CommentDatabaseID]; exists {
			return fmt.Errorf("%s.comment_database_id is duplicated", field)
		}
		databaseIDs[section.CommentDatabaseID] = struct{}{}
		if !validSHA256(section.BodySHA256) {
			return fmt.Errorf("%s.body_sha256 must use lowercase sha256:<64 hex> format", field)
		}
	}
	for _, role := range []SourceSectionRole{
		SourceSectionOutcome,
		SourceSectionRequirements,
		SourceSectionAcceptanceExamples,
		SourceSectionConstraints,
		SourceSectionNonGoals,
	} {
		if roles[role] == 0 {
			return fmt.Errorf("sections require at least one %q role", role)
		}
	}
	return nil
}

func validSourceSectionKey(key string) bool {
	if len(key) == 0 || len(key) > 64 {
		return false
	}
	for index := 0; index < len(key); index++ {
		character := key[index]
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			continue
		}
		if index > 0 && (character == '-' || character == '_' || character == '.') {
			continue
		}
		return false
	}
	return true
}

func validSourceSectionRole(role SourceSectionRole) bool {
	switch role {
	case SourceSectionOutcome,
		SourceSectionRequirements,
		SourceSectionAcceptanceExamples,
		SourceSectionConstraints,
		SourceSectionNonGoals:
		return true
	default:
		return false
	}
}

func parseSourceSection(key, body string) (string, error) {
	lines := strings.Split(body, "\n")
	markerIndex := firstNonemptyLine(lines, 0)
	expectedMarker := sectionMarkerPrefix + key + " -->"
	if markerIndex < 0 || lines[markerIndex] != expectedMarker {
		return "", newSourceBundleError(
			SourceClassificationSectionInvalid,
			fmt.Sprintf("manifest section %q comment must begin with %s", key, expectedMarker),
		)
	}
	payload := strings.Join(lines[markerIndex+1:], "\n")
	if strings.TrimSpace(payload) == "" {
		return "", newSourceBundleError(
			SourceClassificationSectionInvalid,
			fmt.Sprintf("manifest section %q payload must be nonempty", key),
		)
	}
	if !utf8.ValidString(payload) {
		return "", newSourceBundleError(
			SourceClassificationSectionInvalid,
			fmt.Sprintf("manifest section %q payload must be valid utf-8", key),
		)
	}
	if err := validateTextControls("section payload", payload, true); err != nil {
		return "", newSourceBundleError(SourceClassificationSectionInvalid, err.Error())
	}
	return payload, nil
}

func validateRawSourceComment(
	field string,
	comment RawSourceComment,
	issueURL string,
) error {
	if err := validateGitHubNodeID(field+".node_id", comment.NodeID); err != nil {
		return err
	}
	if comment.DatabaseID <= 0 {
		return fmt.Errorf("%s.database_id must be positive", field)
	}
	if err := validateGitHubNodeID(field+".author_id", comment.AuthorID); err != nil {
		return err
	}
	for name, value := range map[string]string{
		field + ".url":        comment.URL,
		field + ".updated_at": comment.UpdatedAt,
	} {
		if err := validateTextControls(name, value, false); err != nil {
			return err
		}
	}
	if err := validateTextControls(field+".body", comment.Body, true); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, comment.UpdatedAt); err != nil {
		return fmt.Errorf("%s.updated_at must be rfc3339: %w", field, err)
	}
	if err := validateGitHubCommentURL(field+".url", comment.URL, issueURL, comment.DatabaseID); err != nil {
		return err
	}
	return nil
}

func validateGitHubCommentURL(field, value, issueURL string, databaseID int64) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid url: %w", field, err)
	}
	expectedFragment := "issuecomment-" + strconv.FormatInt(databaseID, 10)
	if parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.RawQuery != "" || parsed.Fragment != expectedFragment {
		return fmt.Errorf("%s must be an https://github.com issue comment url", field)
	}
	parsed.Fragment = ""
	if parsed.String() != issueURL {
		return fmt.Errorf("%s must belong to the source issue", field)
	}
	return nil
}

func validatePinnedSections(sections []PinnedSourceSection) error {
	if sections == nil {
		return errors.New("sections must be an initialized array")
	}
	if len(sections) == 0 || len(sections) > maxSourceSections {
		return fmt.Errorf("sections must contain 1..%d entries", maxSourceSections)
	}
	keys := make(map[string]struct{}, len(sections))
	nodeIDs := make(map[string]struct{}, len(sections))
	totalBytes := 0
	for index, section := range sections {
		field := fmt.Sprintf("sections[%d]", index)
		if !validSourceSectionKey(section.Key) {
			return fmt.Errorf("%s.key is invalid", field)
		}
		if _, exists := keys[section.Key]; exists {
			return fmt.Errorf("%s.key is duplicated", field)
		}
		keys[section.Key] = struct{}{}
		if !validSourceSectionRole(section.Role) {
			return fmt.Errorf("%s.role is invalid", field)
		}
		if err := validateTextControls(field+".title", section.Title, false); err != nil {
			return err
		}
		if strings.TrimSpace(section.Title) == "" || len(section.Title) > maxSourceSectionTitleBytes {
			return fmt.Errorf("%s.title is invalid", field)
		}
		if !validSHA256(section.BodySHA256) || !validSHA256(section.SectionRevision) || !validSHA256(section.MaterialSHA256) {
			return fmt.Errorf("%s revisions must use lowercase sha256:<64 hex> format", field)
		}
		if section.Bytes <= 0 || section.Bytes > maxSourceSectionBytes {
			return fmt.Errorf("%s.bytes must be 1..%d", field, maxSourceSectionBytes)
		}
		if totalBytes > maxSourceMaterialBytes-section.Bytes {
			return fmt.Errorf("sections exceed %d bytes", maxSourceMaterialBytes)
		}
		totalBytes += section.Bytes
		provenance := section.Provenance
		if err := validateGitHubNodeID(field+".provenance.comment_node_id", provenance.CommentNodeID); err != nil {
			return err
		}
		if _, exists := nodeIDs[provenance.CommentNodeID]; exists {
			return fmt.Errorf("%s.provenance.comment_node_id is duplicated", field)
		}
		nodeIDs[provenance.CommentNodeID] = struct{}{}
		if provenance.CommentDatabaseID <= 0 {
			return fmt.Errorf("%s.provenance.comment_database_id must be positive", field)
		}
		if err := validateGitHubNodeID(field+".provenance.author_id", provenance.AuthorID); err != nil {
			return err
		}
		if err := validateTextControls(field+".provenance.url", provenance.URL, false); err != nil {
			return err
		}
		if _, err := time.Parse(time.RFC3339, provenance.ObservedUpdatedAt); err != nil {
			return fmt.Errorf("%s.provenance.observed_updated_at must be rfc3339: %w", field, err)
		}
	}
	return nil
}

func validateSourceMaterials(source PinnedSource, required bool) error {
	if source.materials == nil {
		if required {
			return errors.New("fresh pinned source requires every local material")
		}
		return nil
	}
	materials := make(map[string][]byte, len(source.materials))
	for _, material := range source.materials {
		if existing, duplicate := materials[material.Digest]; duplicate {
			if string(existing) != string(material.Data) {
				return errors.New("source materials contain conflicting data for one digest")
			}
			continue
		}
		materials[material.Digest] = material.Data
	}
	referenced := make(map[string]struct{}, len(source.Sections))
	for _, section := range source.Sections {
		data, ok := materials[section.MaterialSHA256]
		if !ok {
			return fmt.Errorf("section %q material is missing", section.Key)
		}
		if len(data) != section.Bytes || materialRevision(string(data)) != section.MaterialSHA256 {
			return fmt.Errorf("section %q material does not match catalog", section.Key)
		}
		if sectionRevision(section.Key, section.Role, section.Title, string(data)) != section.SectionRevision {
			return fmt.Errorf("section %q revision does not match material", section.Key)
		}
		referenced[section.MaterialSHA256] = struct{}{}
	}
	for digest := range materials {
		if _, ok := referenced[digest]; !ok {
			return errors.New("source materials contain an unreferenced blob")
		}
	}
	return nil
}

func validateReplacementOnlyAmendment(current, amended PinnedSource) error {
	currentByNode := make(map[string]PinnedSourceSection, len(current.Sections))
	currentByDatabaseID := make(map[int64]PinnedSourceSection, len(current.Sections))
	for _, section := range current.Sections {
		currentByNode[section.Provenance.CommentNodeID] = section
		currentByDatabaseID[section.Provenance.CommentDatabaseID] = section
	}
	for _, section := range amended.Sections {
		if prior, ok := currentByNode[section.Provenance.CommentNodeID]; ok {
			if prior.Provenance.CommentDatabaseID != section.Provenance.CommentDatabaseID {
				return fmt.Errorf(
					"accepted comment node %q was rebound from database id %d to %d",
					section.Provenance.CommentNodeID,
					prior.Provenance.CommentDatabaseID,
					section.Provenance.CommentDatabaseID,
				)
			}
			if !sameAcceptedSection(prior, section) {
				return fmt.Errorf(
					"accepted comment node %q was changed in place; publish a replacement comment",
					section.Provenance.CommentNodeID,
				)
			}
		}
		if prior, ok := currentByDatabaseID[section.Provenance.CommentDatabaseID]; ok &&
			prior.Provenance.CommentNodeID != section.Provenance.CommentNodeID {
			return fmt.Errorf(
				"comment database id %d was rebound from node %q to %q",
				section.Provenance.CommentDatabaseID,
				prior.Provenance.CommentNodeID,
				section.Provenance.CommentNodeID,
			)
		}
	}
	return nil
}

func sameAcceptedSection(left, right PinnedSourceSection) bool {
	return left.Key == right.Key &&
		left.Role == right.Role &&
		left.Title == right.Title &&
		left.BodySHA256 == right.BodySHA256 &&
		left.SectionRevision == right.SectionRevision &&
		left.MaterialSHA256 == right.MaterialSHA256 &&
		left.Bytes == right.Bytes
}

func manifestFromPinnedSource(source PinnedSource) SourceManifest {
	sections := make([]SourceManifestSection, 0, len(source.Sections))
	for _, section := range source.Sections {
		sections = append(sections, SourceManifestSection{
			Key:               section.Key,
			Role:              section.Role,
			Title:             section.Title,
			CommentNodeID:     section.Provenance.CommentNodeID,
			CommentDatabaseID: section.Provenance.CommentDatabaseID,
			BodySHA256:        section.BodySHA256,
		})
	}
	return SourceManifest{
		ManifestVersion:            source.ManifestVersion,
		Profile:                    source.Profile,
		ParentRequirementsRevision: source.ParentRequirementsRevision,
		Sections:                   sections,
	}
}

// ComputeSourceCommentBodySHA256 returns the exact digest a source manifest
// must declare for one normalized GitHub comment body.
func ComputeSourceCommentBodySHA256(body string) (string, error) {
	normalized := normalizeLineEndings(body)
	if err := validateTextControls("comment body", normalized, true); err != nil {
		return "", err
	}
	return commentBodyRevision(normalized), nil
}

func commentBodyRevision(body string) string {
	return framedRevision("slipway-comment-body/v1", body)
}

func materialRevision(payload string) string {
	return framedRevision("slipway-material/v1", payload)
}

func sectionRevision(key string, role SourceSectionRole, title, payload string) string {
	return framedRevision("slipway-section/v2", key, string(role), title, payload)
}

func manifestRevision(manifest SourceManifest) string {
	fields := []string{
		"slipway-manifest/v2",
		strconv.Itoa(manifest.ManifestVersion),
		manifest.Profile,
		manifest.ParentRequirementsRevision,
	}
	for _, section := range manifest.Sections {
		fields = append(fields,
			section.Key,
			string(section.Role),
			section.Title,
			section.CommentNodeID,
			strconv.FormatInt(section.CommentDatabaseID, 10),
			section.BodySHA256,
		)
	}
	return framedRevision(fields...)
}

func requirementsRevision(profile string, sections []PinnedSourceSection) string {
	fields := []string{"slipway-requirements/v2", profile}
	for _, section := range sections {
		fields = append(fields,
			section.Key,
			string(section.Role),
			section.Title,
			section.SectionRevision,
		)
	}
	return framedRevision(fields...)
}

func sourceRevision(envelope RawSourceEnvelope, manifestSHA256 string) string {
	return sourceRevisionFromIdentity(
		envelope.Host,
		envelope.RepositoryID,
		envelope.IssueID,
		envelope.Title,
		manifestSHA256,
	)
}

func sourceRevisionFromIdentity(host, repositoryID, issueID, title, manifestSHA256 string) string {
	return framedRevision(
		"slipway-source/v2",
		strconv.Itoa(SourceVersion),
		host,
		repositoryID,
		issueID,
		normalizeLineEndings(title),
		manifestSHA256,
	)
}

func observedSourceRevision(envelope RawSourceEnvelope, normalizedIssueBody string) string {
	comments := append([]RawSourceComment(nil), envelope.Comments...)
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].NodeID < comments[j].NodeID
	})
	fields := []string{
		"slipway-source-observation/v2",
		envelope.Host,
		envelope.RepositoryID,
		envelope.IssueID,
		normalizeLineEndings(envelope.Title),
		normalizedIssueBody,
	}
	for _, comment := range comments {
		fields = append(fields, comment.NodeID, commentBodyRevision(normalizeLineEndings(comment.Body)))
	}
	return framedRevision(fields...)
}
