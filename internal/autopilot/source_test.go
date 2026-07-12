package autopilot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSourcePreservesExactNormalizedSectionsAndStableRevisions(t *testing.T) {
	t.Parallel()

	envelope := validSourceEnvelope()
	pinned := mustParseSource(t, envelope)

	assert.Equal(t, SourceVersion, pinned.SourceVersion)
	assert.Equal(t, ParserVersion, pinned.ParserVersion)
	assert.Equal(t, envelope.RepositoryID, pinned.RepositoryID)
	assert.Equal(t, envelope.IssueID, pinned.IssueID)
	assert.Equal(t, envelope.IssueNumber, pinned.IssueNumber)
	assert.Equal(t, envelope.CanonicalURL, pinned.CanonicalURL)
	assert.NotNil(t, pinned.URLAliases)
	assert.Empty(t, pinned.URLAliases)
	require.NotNil(t, pinned.Parent)
	assert.Equal(t, *envelope.Parent, *pinned.Parent)
	assert.Equal(t, "\nDeliver **value**.  \n`literal`\n\n", pinned.AcceptedRequirements.OutcomeMarkdown)
	assert.Equal(t, "\n- Keep order.\n- Preserve  spaces.\n\n", pinned.AcceptedRequirements.RequirementsMarkdown)
	assert.Equal(t, "\n```text\n## Requirements\n<!-- slipway-level: objective/v1 -->\n```\n\n", pinned.AcceptedRequirements.AcceptanceExamplesMarkdown)
	assert.Equal(t, "\n\tTabbed body text\n\n", pinned.AcceptedRequirements.ConstraintsMarkdown)
	assert.Equal(t, "\nNone.\n\n", pinned.AcceptedRequirements.NonGoalsMarkdown)
	assert.Equal(t, "sha256:051004b9762527da3bb73e3b3320fcacbdf96ada573c7edbb445167df209b4f8", pinned.SourceRevision)
	assert.Equal(t, "sha256:83a06a3a4d11cdbe5a67e89758d63df118455b2193527945e5471ca4ce89dd4a", pinned.RequirementsRevision)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, pinned.SourceRevision)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, pinned.RequirementsRevision)

	lfEnvelope := validSourceEnvelope()
	lfEnvelope.Body = normalizeLineEndings(lfEnvelope.Body)
	lfPinned := mustParseSource(t, lfEnvelope)
	assert.Equal(t, pinned.SourceRevision, lfPinned.SourceRevision)
	assert.Equal(t, pinned.RequirementsRevision, lfPinned.RequirementsRevision)
	assert.Equal(t, pinned.AcceptedRequirements, lfPinned.AcceptedRequirements)

	for range 20 {
		repeated := mustParseSource(t, validSourceEnvelope())
		assert.Equal(t, pinned.SourceRevision, repeated.SourceRevision)
		assert.Equal(t, pinned.RequirementsRevision, repeated.RequirementsRevision)
	}
}

func TestSourceRevisionsTrackOnlyNormativeInputs(t *testing.T) {
	t.Parallel()

	baseline := mustParseSource(t, validSourceEnvelope())
	tests := []struct {
		name               string
		mutate             func(*RawSourceEnvelope)
		sourceChanges      bool
		requirementsChange bool
	}{
		{
			name: "accepted requirement text",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Body = strings.Replace(envelope.Body, "Deliver **value**.", "Deliver **more value**.", 1)
			},
			sourceChanges:      true,
			requirementsChange: true,
		},
		{
			name: "implementation checklist",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Body = strings.Replace(envelope.Body, "Internal step", "Different internal step", 1)
			},
			sourceChanges: true,
		},
		{
			name: "other excluded section",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Body += "\r\n## Notes\r\nExcluded detail\r\n"
			},
			sourceChanges: true,
		},
		{
			name: "title",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Title = "[Change] A different title"
			},
			sourceChanges: true,
		},
		{
			name: "repository node id",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.RepositoryID = "R_kgDOTransferred"
			},
			sourceChanges: true,
		},
		{
			name: "issue node id",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.IssueID = "I_kwDOTransferred"
			},
			sourceChanges: true,
		},
		{
			name: "labels",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Labels = append(envelope.Labels, "ready-for-agent")
			},
		},
		{
			name: "timestamps",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.UpdatedAt = "2026-07-12T10:30:00Z"
				envelope.FetchedAt = "2026-07-12T10:31:00Z"
			},
		},
		{
			name: "url projection",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.CanonicalURL = "https://github.com/other-owner/slipway/issues/42"
			},
		},
		{
			name: "issue number projection",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.IssueNumber = 84
				envelope.CanonicalURL = "https://github.com/signalridge/slipway/issues/84"
			},
		},
		{
			name: "parent projection",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Parent = &SourceParent{RepositoryID: "R_parent_new", IssueID: "I_parent_new", CanonicalURL: "https://github.com/signalridge/slipway/issues/41"}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			envelope := validSourceEnvelope()
			test.mutate(&envelope)
			pinned := mustParseSource(t, envelope)
			if test.sourceChanges {
				assert.NotEqual(t, baseline.SourceRevision, pinned.SourceRevision)
			} else {
				assert.Equal(t, baseline.SourceRevision, pinned.SourceRevision)
			}
			if test.requirementsChange {
				assert.NotEqual(t, baseline.RequirementsRevision, pinned.RequirementsRevision)
			} else {
				assert.Equal(t, baseline.RequirementsRevision, pinned.RequirementsRevision)
			}
		})
	}
}

func TestParseSourceDoesNotUnicodeNormalizeAcceptedText(t *testing.T) {
	t.Parallel()

	composed := validSourceEnvelope()
	composed.Body = strings.Replace(composed.Body, "Deliver **value**.", "Deliver **caf\u00e9**.", 1)
	decomposed := validSourceEnvelope()
	decomposed.Body = strings.Replace(decomposed.Body, "Deliver **value**.", "Deliver **cafe\u0301**.", 1)

	composedPinned := mustParseSource(t, composed)
	decomposedPinned := mustParseSource(t, decomposed)
	assert.NotEqual(t, composedPinned.SourceRevision, decomposedPinned.SourceRevision)
	assert.NotEqual(t, composedPinned.RequirementsRevision, decomposedPinned.RequirementsRevision)
	assert.Contains(t, composedPinned.AcceptedRequirements.OutcomeMarkdown, "caf\u00e9")
	assert.Contains(t, decomposedPinned.AcceptedRequirements.OutcomeMarkdown, "cafe\u0301")
}

func TestParseSourceSupportsOmittedParent(t *testing.T) {
	t.Parallel()

	envelope := validSourceEnvelope()
	envelope.Parent = nil
	pinned := mustParseSource(t, envelope)
	assert.Nil(t, pinned.Parent)
	assert.NotNil(t, pinned.URLAliases)
}

func TestParseSourceRejectsInvalidIdentityProjectionAndText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*RawSourceEnvelope)
		want   string
	}{
		{name: "source version", mutate: func(value *RawSourceEnvelope) { value.SourceVersion = 2 }, want: "source_version"},
		{name: "provider", mutate: func(value *RawSourceEnvelope) { value.Provider = "gitlab" }, want: "provider"},
		{name: "host", mutate: func(value *RawSourceEnvelope) { value.Host = "www.github.com" }, want: "host"},
		{name: "empty repository id", mutate: func(value *RawSourceEnvelope) { value.RepositoryID = "" }, want: "repository_id"},
		{name: "whitespace repository id", mutate: func(value *RawSourceEnvelope) { value.RepositoryID = "R_ bad" }, want: "repository_id"},
		{name: "empty issue id", mutate: func(value *RawSourceEnvelope) { value.IssueID = "" }, want: "issue_id"},
		{name: "nonpositive issue number", mutate: func(value *RawSourceEnvelope) { value.IssueNumber = 0 }, want: "issue_number"},
		{name: "empty title", mutate: func(value *RawSourceEnvelope) { value.Title = " \t" }, want: "title"},
		{name: "title control", mutate: func(value *RawSourceEnvelope) { value.Title = "bad\u0001title" }, want: "c0 control"},
		{name: "body nul", mutate: func(value *RawSourceEnvelope) { value.Body += "\x00" }, want: "c0 control"},
		{name: "body form feed", mutate: func(value *RawSourceEnvelope) { value.Body += "\f" }, want: "c0 control"},
		{name: "updated time", mutate: func(value *RawSourceEnvelope) { value.UpdatedAt = "yesterday" }, want: "updated_at"},
		{name: "fetched time", mutate: func(value *RawSourceEnvelope) { value.FetchedAt = "2026-07-12" }, want: "fetched_at"},
		{name: "http url", mutate: func(value *RawSourceEnvelope) { value.CanonicalURL = "http://github.com/signalridge/slipway/issues/42" }, want: "https://github.com"},
		{name: "wrong url host", mutate: func(value *RawSourceEnvelope) {
			value.CanonicalURL = "https://example.com/signalridge/slipway/issues/42"
		}, want: "https://github.com"},
		{name: "url userinfo", mutate: func(value *RawSourceEnvelope) {
			value.CanonicalURL = "https://token@github.com/signalridge/slipway/issues/42"
		}, want: "userinfo"},
		{name: "url query", mutate: func(value *RawSourceEnvelope) { value.CanonicalURL += "?state=open" }, want: "query or fragment"},
		{name: "url fragment", mutate: func(value *RawSourceEnvelope) { value.CanonicalURL += "#issue" }, want: "query or fragment"},
		{name: "url nondefault port", mutate: func(value *RawSourceEnvelope) {
			value.CanonicalURL = "https://github.com:444/signalridge/slipway/issues/42"
		}, want: "non-default port"},
		{name: "url empty port", mutate: func(value *RawSourceEnvelope) {
			value.CanonicalURL = "https://github.com:/signalridge/slipway/issues/42"
		}, want: "exact github.com host"},
		{name: "url shape", mutate: func(value *RawSourceEnvelope) { value.CanonicalURL = "https://github.com/signalridge/slipway/pull/42" }, want: "must match"},
		{name: "url number mismatch", mutate: func(value *RawSourceEnvelope) {
			value.CanonicalURL = "https://github.com/signalridge/slipway/issues/43"
		}, want: "match issue_number"},
		{name: "url noncanonical number", mutate: func(value *RawSourceEnvelope) {
			value.CanonicalURL = "https://github.com/signalridge/slipway/issues/042"
		}, want: "canonical decimal"},
		{name: "missing labels", mutate: func(value *RawSourceEnvelope) { value.Labels = nil }, want: "labels"},
		{name: "empty label", mutate: func(value *RawSourceEnvelope) { value.Labels = []string{""} }, want: "labels[0]"},
		{name: "label control", mutate: func(value *RawSourceEnvelope) { value.Labels = []string{"kind:bug\nother"} }, want: "c0 control"},
		{name: "parent repository id", mutate: func(value *RawSourceEnvelope) { value.Parent.RepositoryID = "" }, want: "parent.repository_id"},
		{name: "parent issue id", mutate: func(value *RawSourceEnvelope) { value.Parent.IssueID = "I_ parent" }, want: "parent.issue_id"},
		{name: "parent url", mutate: func(value *RawSourceEnvelope) {
			value.Parent.CanonicalURL = "https://github.com/signalridge/slipway/issues/0"
		}, want: "positive canonical decimal"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			envelope := validSourceEnvelope()
			test.mutate(&envelope)
			raw, err := json.Marshal(envelope)
			require.NoError(t, err)
			_, err = ParseSource(raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestParseSourceRejectsWrongJSONTypes(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(validSourceEnvelope())
	require.NoError(t, err)
	wrongType := strings.Replace(string(raw), `"issue_number":42`, `"issue_number":"42"`, 1)
	_, err = ParseSource([]byte(wrongType))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal")
}

func TestParseSourceRejectsEachMissingAcceptedHeading(t *testing.T) {
	t.Parallel()

	for _, heading := range acceptedSectionHeadings {
		t.Run(heading, func(t *testing.T) {
			t.Parallel()
			envelope := validSourceEnvelope()
			envelope.Body = strings.Replace(envelope.Body, "## "+heading+"\r\n", "", 1)
			raw, err := json.Marshal(envelope)
			require.NoError(t, err)
			_, err = ParseSource(raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing accepted h2 heading")
		})
	}
}

func TestParseSourceRejectsInvalidMarkersAndAcceptedHeadings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{name: "objective marker", mutate: func(body string) string {
			return strings.Replace(body, changeSourceMarker, "<!-- slipway-level: objective/v1 -->", 1)
		}, want: "objective marker"},
		{name: "unknown marker", mutate: func(body string) string {
			return strings.Replace(body, changeSourceMarker, "<!-- slipway-level: change/v2 -->", 1)
		}, want: "unsupported"},
		{name: "multiple markers", mutate: func(body string) string { return body + "\r\n<!-- slipway-level: change/v1 -->\r\n" }, want: "multiple"},
		{name: "content before marker", mutate: func(body string) string { return "Introduction\r\n" + body }, want: "first nonempty"},
		{name: "missing heading", mutate: func(body string) string { return strings.Replace(body, "## Constraints\r\n", "", 1) }, want: "missing accepted"},
		{name: "duplicate heading", mutate: func(body string) string { return body + "\r\n## Outcome\r\nAgain\r\n" }, want: "duplicate accepted"},
		{name: "ambiguous trailing space", mutate: func(body string) string { return strings.Replace(body, "## Constraints\r\n", "## Constraints \r\n", 1) }, want: "ambiguous accepted"},
		{name: "ambiguous heading case", mutate: func(body string) string { return strings.Replace(body, "## Non-goals\r\n", "## non-goals\r\n", 1) }, want: "ambiguous accepted"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			envelope := validSourceEnvelope()
			envelope.Body = test.mutate(envelope.Body)
			raw, err := json.Marshal(envelope)
			require.NoError(t, err)
			_, err = ParseSource(raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestParseSourceIgnoresMarkerAndHeadingsInsideFences(t *testing.T) {
	t.Parallel()

	pinned := mustParseSource(t, validSourceEnvelope())
	assert.Contains(t, pinned.AcceptedRequirements.AcceptanceExamplesMarkdown, "<!-- slipway-level: objective/v1 -->")
	assert.Contains(t, pinned.AcceptedRequirements.AcceptanceExamplesMarkdown, "## Requirements")
}

func TestParseSourceEnforcesAcceptedRequirementsByteLimit(t *testing.T) {
	t.Parallel()

	envelope := validSourceEnvelope()
	envelope.Body = sourceBodyWithOutcomeBytes(maxAcceptedRequirementsBytes)
	pinned := mustParseSource(t, envelope)
	assert.Len(t, pinned.AcceptedRequirements.OutcomeMarkdown, maxAcceptedRequirementsBytes)

	envelope.Body = sourceBodyWithOutcomeBytes(maxAcceptedRequirementsBytes + 1)
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	_, err = ParseSource(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepted requirements exceed")
}

func TestImportSourceFileAcceptsAbsoluteAndRelativeReadOnlyFiles(t *testing.T) {
	raw, err := json.Marshal(validSourceEnvelope())
	require.NoError(t, err)
	directory := t.TempDir()
	path := filepath.Join(directory, "source.json")
	require.NoError(t, os.WriteFile(path, raw, 0o400))

	absolutePinned, err := ImportSourceFile(path)
	require.NoError(t, err)
	t.Chdir(directory)
	relativePinned, err := ImportSourceFile(filepath.Base(path))
	require.NoError(t, err)
	assert.Equal(t, absolutePinned, relativePinned)
	require.NoError(t, os.Remove(path), "the importer must close the opened source handle")
}

func TestImportSourceFileRejectsOversizedAndNonRegularFiles(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	oversized := filepath.Join(directory, "oversized.json")
	require.NoError(t, os.WriteFile(oversized, []byte(strings.Repeat(" ", maxSourceFileBytes+1)), 0o600))

	validRaw, err := json.Marshal(validSourceEnvelope())
	require.NoError(t, err)
	target := filepath.Join(directory, "target.json")
	require.NoError(t, os.WriteFile(target, validRaw, 0o600))
	link := filepath.Join(directory, "source-link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "oversized", path: oversized, want: "exceeds"},
		{name: "symlink", path: link, want: "not a regular file"},
		{name: "directory", path: directory, want: "not a regular file"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := ImportSourceFile(test.path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestPinnedSourceJSONExcludesEphemeralRawFields(t *testing.T) {
	t.Parallel()

	pinned := mustParseSource(t, validSourceEnvelope())
	raw, err := json.Marshal(pinned)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))

	for _, excluded := range []string{"body", "comments", "updated_at", "fetched_at", "labels"} {
		assert.NotContains(t, fields, excluded)
	}
	assert.JSONEq(t, `[]`, string(fields["url_aliases"]))
	assert.NotContains(t, string(raw), "Internal step")

	var parentFields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(fields["parent"], &parentFields))
	assert.NotContains(t, parentFields, "issue_number")
	assert.Len(t, parentFields, 3)
}

func sourceBodyWithOutcomeBytes(outcomeBytes int) string {
	return changeSourceMarker + "\n## Outcome\n" + strings.Repeat("x", outcomeBytes-1) + "\n" +
		"## Requirements\n" +
		"## Acceptance examples\n" +
		"## Constraints\n" +
		"## Non-goals"
}

func validSourceEnvelope() RawSourceEnvelope {
	return RawSourceEnvelope{
		SourceVersion: SourceVersion,
		Provider:      "github",
		Host:          "github.com",
		RepositoryID:  "R_kgDOExample",
		IssueID:       "I_kwDOExample",
		IssueNumber:   42,
		CanonicalURL:  "https://github.com/signalridge/slipway/issues/42",
		UpdatedAt:     "2026-07-12T09:00:00Z",
		FetchedAt:     "2026-07-12T09:01:00Z",
		Title:         "[Change] Preserve source requirements",
		Body: "\r\n" + changeSourceMarker + "\r\n\r\n" +
			"## Outcome\r\n\r\n" +
			"Deliver **value**.  \r\n" +
			"`literal`\r\n\r\n" +
			"## Requirements\r\n\r\n" +
			"- Keep order.\r\n" +
			"- Preserve  spaces.\r\n\r\n" +
			"## Acceptance examples\r\n\r\n" +
			"```text\r\n" +
			"## Requirements\r\n" +
			"<!-- slipway-level: objective/v1 -->\r\n" +
			"```\r\n\r\n" +
			"## Constraints\r\n\r\n" +
			"\tTabbed body text\r\n\r\n" +
			"## Non-goals\r\n\r\n" +
			"None.\r\n\r\n" +
			"## Implementation checklist\r\n\r\n" +
			"- [ ] Internal step\r\n",
		Labels: []string{"level:change", "kind:refactor"},
		Parent: &SourceParent{
			RepositoryID: "R_kgDOExample",
			IssueID:      "I_kwDOParent",
			CanonicalURL: "https://github.com/signalridge/slipway/issues/40",
		},
	}
}

func mustParseSource(t *testing.T, envelope RawSourceEnvelope) PinnedSource {
	t.Helper()
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	pinned, err := ParseSource(raw)
	require.NoError(t, err)
	return pinned
}

func TestParseSourceCandidateSeparatesIdentityFailureFromBodyClassification(t *testing.T) {
	t.Parallel()

	validEnvelope := validSourceEnvelope()
	validRaw, err := json.Marshal(validEnvelope)
	require.NoError(t, err)
	valid, err := ParseSourceCandidate(validRaw)
	require.NoError(t, err)
	assert.True(t, valid.Valid)
	assert.Equal(t, SourceClassificationValid, valid.Classification)
	assert.Equal(t, SourceClassificationValidChange, valid.ClassificationCode)
	assert.Empty(t, valid.ClassificationError)
	require.NotNil(t, valid.Snapshot)
	assert.Equal(t, valid.RequirementsRevision, valid.Snapshot.RequirementsRevision)

	valid.URLAliases = append(valid.URLAliases, "https://github.com/signalridge/slipway/issues/41")
	assert.Empty(t, valid.Snapshot.URLAliases, "candidate projection and normalized snapshot must not share slices")
	valid.Parent.IssueID = "I_kwDOMutated"
	assert.Equal(t, validEnvelope.Parent.IssueID, valid.Snapshot.Parent.IssueID, "candidate projection and snapshot must not share parent pointers")

	tests := []struct {
		name string
		body func(string) string
		code string
	}{
		{
			name: "objective marker",
			body: func(body string) string {
				return strings.Replace(body, changeSourceMarker, "<!-- slipway-level: objective/v1 -->", 1)
			},
			code: SourceClassificationObjectiveMarker,
		},
		{
			name: "missing section",
			body: func(body string) string {
				return strings.Replace(body, "## Constraints\r\n", "", 1)
			},
			code: SourceClassificationAcceptedSectionMissing,
		},
		{
			name: "multiple markers",
			body: func(body string) string {
				return body + "\r\n<!-- slipway-level: change/v1 -->\r\n"
			},
			code: SourceClassificationMultipleMarkers,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope := validSourceEnvelope()
			envelope.Body = test.body(envelope.Body)
			raw, marshalErr := json.Marshal(envelope)
			require.NoError(t, marshalErr)

			candidate, parseErr := ParseSourceCandidate(raw)
			require.NoError(t, parseErr)
			assert.False(t, candidate.Valid)
			assert.Equal(t, SourceClassificationInvalid, candidate.Classification)
			assert.Equal(t, test.code, candidate.ClassificationCode)
			assert.NotEmpty(t, candidate.ClassificationError)
			assert.True(t, validSHA256(candidate.SourceRevision))
			assert.Empty(t, candidate.RequirementsRevision)
			assert.Nil(t, candidate.Snapshot)

			persisted, marshalErr := json.Marshal(candidate)
			require.NoError(t, marshalErr)
			serialized := string(persisted)
			assert.NotContains(t, serialized, `"body"`)
			assert.NotContains(t, serialized, `"labels"`)
			assert.NotContains(t, serialized, `"updated_at"`)
			assert.NotContains(t, serialized, `"fetched_at"`)
			assert.NotContains(t, serialized, `"requirements_revision"`)
			assert.NotContains(t, serialized, `"snapshot"`)
			assert.NotContains(t, serialized, changeSourceMarker)
			assert.NotContains(t, serialized, "Implementation checklist")
		})
	}

	invalidIdentity := validSourceEnvelope()
	invalidIdentity.IssueID = ""
	invalidRaw, err := json.Marshal(invalidIdentity)
	require.NoError(t, err)
	_, err = ParseSourceCandidate(invalidRaw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "issue_id")
}

func TestImportSourceCandidateFileReadsOnceAndDoesNotRetainPath(t *testing.T) {
	t.Parallel()

	envelope := validSourceEnvelope()
	envelope.Body = strings.Replace(envelope.Body, changeSourceMarker, "<!-- slipway-level: objective/v1 -->", 1)
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "ephemeral-source.json")
	require.NoError(t, os.WriteFile(path, raw, 0o400))

	candidate, err := ImportSourceCandidateFile(path)
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))
	assert.False(t, candidate.Valid)
	persisted, err := json.Marshal(candidate)
	require.NoError(t, err)
	assert.NotContains(t, string(persisted), path)
	assert.NotContains(t, string(persisted), filepath.Base(path))
}
