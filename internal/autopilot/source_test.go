package autopilot

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSourceBuildsManifestAddressedBundle(t *testing.T) {
	t.Parallel()
	envelope := validSourceEnvelope()
	pinned := mustParseSource(t, envelope)

	assert.Equal(t, SourceVersion, pinned.SourceVersion)
	assert.Equal(t, ParserVersion, pinned.ParserVersion)
	assert.Equal(t, SourceManifestVersion, pinned.ManifestVersion)
	assert.Equal(t, SourceProfileChangeV2, pinned.Profile)
	require.Len(t, pinned.Sections, 5)
	assert.Equal(t, []string{
		"outcome", "requirements", "acceptance-examples", "constraints", "non-goals",
	}, sourceSectionKeys(pinned.Sections))
	assert.Len(t, pinned.materials, 5)
	assert.Equal(t, "# Outcome\n\nDeliver **value**.  \n`literal`\n", string(pinned.materials[0].Data))
	assert.True(t, validSHA256(pinned.SourceRevision))
	assert.True(t, validSHA256(pinned.ManifestRevision))
	assert.True(t, validSHA256(pinned.RequirementsRevision))
	for index, section := range pinned.Sections {
		assert.Equal(t, pinned.materials[index].Digest, section.MaterialSHA256)
		assert.Equal(t, len(pinned.materials[index].Data), section.Bytes)
	}
}

func TestParseSourceAllowsSectionsToShareContentAddressedMaterial(t *testing.T) {
	t.Parallel()
	envelope := validSourceEnvelope()
	payload := "\n# Shared normative text\n\nThe same bytes may be reused.\n"
	setEnvelopeSection(t, &envelope, "outcome", payload)
	setEnvelopeSection(t, &envelope, "requirements", payload)

	pinned := mustParseSource(t, envelope)
	require.NoError(t, validateSourceMaterials(pinned, true))
	assert.Equal(t, pinned.Sections[0].MaterialSHA256, pinned.Sections[1].MaterialSHA256)
	assert.NotEqual(t, pinned.Sections[0].SectionRevision, pinned.Sections[1].SectionRevision)
}

func TestParseSourceRejectsUnreferencedAndTamperedComments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*RawSourceEnvelope)
		want   string
	}{
		{
			name: "unreferenced discussion comment",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Comments = append(envelope.Comments, RawSourceComment{
					NodeID:     "IC_discussion",
					DatabaseID: 999,
					URL:        envelope.CanonicalURL + "#issuecomment-999",
					UpdatedAt:  "2026-07-12T09:00:00Z",
					AuthorID:   "U_discussion",
					Body:       "ordinary discussion",
				})
			},
			want: "not referenced by the manifest",
		},
		{
			name: "missing referenced comment",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Comments = envelope.Comments[1:]
			},
			want: "missing its referenced comment",
		},
		{
			name: "edited body",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Comments[0].Body += "edited without publishing a manifest"
			},
			want: "digest does not match",
		},
		{
			name: "minimized comment",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Comments[0].IsMinimized = true
			},
			want: "minimized",
		},
		{
			name: "wrong section marker",
			mutate: func(envelope *RawSourceEnvelope) {
				oldDigest := commentBodyRevision(envelope.Comments[0].Body)
				envelope.Comments[0].Body = strings.Replace(
					envelope.Comments[0].Body,
					"key=outcome",
					"key=other",
					1,
				)
				newDigest := commentBodyRevision(envelope.Comments[0].Body)
				envelope.Body = strings.Replace(envelope.Body, oldDigest, newDigest, 1)
			},
			want: "must begin",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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

func TestSourceRevisionSeparatesProjectionManifestAndRequirements(t *testing.T) {
	t.Parallel()
	baseline := mustParseSource(t, validSourceEnvelope())

	title := validSourceEnvelope()
	title.Title = "[Change] Renamed projection"
	titlePinned := mustParseSource(t, title)
	assert.NotEqual(t, baseline.SourceRevision, titlePinned.SourceRevision)
	assert.Equal(t, baseline.RequirementsRevision, titlePinned.RequirementsRevision)

	provenance := validSourceEnvelope()
	provenance.Comments[0].UpdatedAt = "2026-07-13T10:00:00Z"
	provenancePinned := mustParseSource(t, provenance)
	assert.Equal(t, baseline.SourceRevision, provenancePinned.SourceRevision)
	assert.Equal(t, baseline.RequirementsRevision, provenancePinned.RequirementsRevision)

	amended := validSourceEnvelope()
	setEnvelopeSection(t, &amended, "requirements", "\n# Requirements\n\n- Keep amended order.\n")
	amendedPinned := mustParseSource(t, amended)
	assert.NotEqual(t, baseline.SourceRevision, amendedPinned.SourceRevision)
	assert.NotEqual(t, baseline.ManifestRevision, amendedPinned.ManifestRevision)
	assert.NotEqual(t, baseline.RequirementsRevision, amendedPinned.RequirementsRevision)
}

func TestParseSourceEnforcesSectionAndBundleLimits(t *testing.T) {
	t.Parallel()
	envelope := validSourceEnvelope()
	setEnvelopeSection(t, &envelope, "outcome", "\n"+strings.Repeat("x", maxSourceSectionBytes))
	pinned := mustParseSource(t, envelope)
	assert.Equal(t, maxSourceSectionBytes, pinned.Sections[0].Bytes)

	setEnvelopeSection(t, &envelope, "outcome", "\n"+strings.Repeat("x", maxSourceSectionBytes+1))
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	_, err = ParseSource(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestParseSourceRejectsV1AndMalformedManifest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*RawSourceEnvelope)
		want   string
	}{
		{name: "v1 source version", mutate: func(value *RawSourceEnvelope) { value.SourceVersion = 1 }, want: "source_version must be 2"},
		{name: "v1 marker", mutate: func(value *RawSourceEnvelope) {
			value.Body = strings.Replace(value.Body, changeSourceMarker, "<!-- slipway-level: change/v1 -->", 1)
		}, want: "must begin"},
		{name: "objective marker", mutate: func(value *RawSourceEnvelope) {
			value.Body = strings.Replace(value.Body, changeSourceMarker, "<!-- slipway-level: objective/v1 -->", 1)
		}, want: "must begin"},
		{name: "extra level marker after manifest fence", mutate: func(value *RawSourceEnvelope) {
			// Issue #434 §4.2: a managed Change must not contain any
			// slipway-level marker outside the opening marker and manifest
			// fence. Append a conflicting marker after the manifest fence.
			value.Body = value.Body + "\n\n<!-- slipway-level: objective/v1 -->\n"
		}, want: "additional slipway-level marker"},
		{name: "duplicate comment id", mutate: func(value *RawSourceEnvelope) {
			value.Comments[1].NodeID = value.Comments[0].NodeID
		}, want: "duplicated"},
		{name: "comment from another issue", mutate: func(value *RawSourceEnvelope) {
			value.Comments[0].URL = "https://github.com/signalridge/slipway/issues/41#issuecomment-101"
		}, want: "belong to the source issue"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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

func TestParseSourceAllowsLevelMarkerExamplesInsideMarkdownFences(t *testing.T) {
	t.Parallel()
	envelope := validSourceEnvelope()
	envelope.Body += "\n\n````markdown\n```html\n<!-- slipway-level: objective/v1 -->\n```\n````\n"
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	_, err = ParseSource(raw)
	require.NoError(t, err)
}

func TestParseSourceAllowsLevelMarkerExamplesInsideGFMContainers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		markdown string
	}{
		{
			name:     "block quote",
			markdown: "> ```markdown\n> <!-- slipway-level: objective/v1 -->\n> ```",
		},
		{
			name:     "list item",
			markdown: "- ~~~markdown\n  <!-- slipway-level: objective/v1 -->\n  ~~~",
		},
		{
			name:     "quoted list item",
			markdown: "> - ````markdown\n>   <!-- slipway-level: objective/v1 -->\n>   ````",
		},
		{
			name:     "list continuation",
			markdown: "- Example:\n\n    ```markdown\n    <!-- slipway-level: objective/v1 -->\n    ```",
		},
		{
			name:     "two-space list continuation",
			markdown: "- Example:\n  ```markdown\n  <!-- slipway-level: objective/v1 -->\n  ```",
		},
		{
			name:     "quoted list continuation",
			markdown: "> - Example:\n>\n>     ~~~markdown\n>     <!-- slipway-level: objective/v1 -->\n>     ~~~",
		},
		{
			name:     "blockquote nested in list item",
			markdown: "- > ```markdown\n  > <!-- slipway-level: objective/v1 -->\n  > ```",
		},
		{
			name:     "deeply nested list continuation",
			markdown: "- outer\n  - inner\n    - Example:\n        ```markdown\n      <!-- slipway-level: objective/v1 -->\n        ```",
		},
		{
			name:     "sibling blockquote after blank line",
			markdown: "> - old item\n\n>   ~~~markdown\n> <!-- slipway-level: objective/v1 -->\n> ~~~",
		},
		{
			name:     "ordered list after blank line",
			markdown: "2. ~~~markdown\n   <!-- slipway-level: objective/v1 -->\n   ~~~",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope := validSourceEnvelope()
			envelope.Body += "\n\n" + test.markdown + "\n"
			raw, err := json.Marshal(envelope)
			require.NoError(t, err)
			_, err = ParseSource(raw)
			require.NoError(t, err)
		})
	}
}

func TestParseSourceRejectsMarkerDedentedOutOfContainerFence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		markdown string
	}{
		{
			name:     "list",
			markdown: "- Example:\n  ```markdown\n<!-- slipway-level: objective/v1 -->\n  ```",
		},
		{
			name:     "blockquote nested in list",
			markdown: "- Example:\n\n  > ```markdown\n> <!-- slipway-level: objective/v1 -->\n  > ```",
		},
		{
			name:     "fence dedented from nested to outer list",
			markdown: "- outer\n  - inner\n\n  ```markdown\n  sample\n    ```\n<!-- slipway-level: objective/v1 -->",
		},
		{
			name:     "stale quoted-list context",
			markdown: "> - old item\n\n>   ~~~markdown\n>   sample\n> ~~~\n> <!-- slipway-level: objective/v1 -->\n> ~~~",
		},
		{
			name:     "non-one ordered list cannot interrupt paragraph",
			markdown: "paragraph\n2. ~~~markdown\n   <!-- slipway-level: objective/v1 -->\n   ~~~",
		},
		{
			name:     "zero-start ordered list cannot interrupt paragraph",
			markdown: "paragraph\n0. ~~~markdown\n   <!-- slipway-level: objective/v1 -->\n   ~~~",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope := validSourceEnvelope()
			envelope.Body += "\n\n" + test.markdown + "\n"
			raw, err := json.Marshal(envelope)
			require.NoError(t, err)
			_, err = ParseSource(raw)
			require.ErrorContains(t, err, "additional slipway-level marker outside a code fence")
		})
	}
}

func TestValidateSourceManifestReportsFiveSectionMinimum(t *testing.T) {
	t.Parallel()
	envelope := validSourceEnvelope()
	manifest, err := parseSourceManifest(envelope.Body)
	require.NoError(t, err)
	manifest.Sections = manifest.Sections[:4]
	err = validateSourceManifest(manifest)
	require.ErrorContains(t, err, "sections must contain 5..64 entries")
}

func TestParseSourceRejectsDELAndC1Controls(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RawSourceEnvelope)
	}{
		{name: "title del", mutate: func(envelope *RawSourceEnvelope) { envelope.Title += "\u007f" }},
		{name: "label c1", mutate: func(envelope *RawSourceEnvelope) { envelope.Labels[0] += "\u0085" }},
		{name: "section body c1", mutate: func(envelope *RawSourceEnvelope) { envelope.Comments[0].Body += "\u009f" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope := validSourceEnvelope()
			test.mutate(&envelope)
			raw, err := json.Marshal(envelope)
			require.NoError(t, err)
			_, err = ParseSource(raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "disallowed control")
		})
	}
}

func TestParseSourceRejectsExplicitDefaultPortsConsistently(t *testing.T) {
	tests := []struct {
		name   string
		want   string
		mutate func(*RawSourceEnvelope)
	}{
		{name: "issue url", want: "explicit port", mutate: func(envelope *RawSourceEnvelope) {
			envelope.CanonicalURL = strings.Replace(envelope.CanonicalURL, "github.com/", "github.com:443/", 1)
		}},
		{name: "comment url", want: "issue comment url", mutate: func(envelope *RawSourceEnvelope) {
			envelope.Comments[0].URL = strings.Replace(envelope.Comments[0].URL, "github.com/", "github.com:443/", 1)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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

func TestSourceProjectionCollectionsAreBounded(t *testing.T) {
	t.Parallel()

	envelope := validSourceEnvelope()
	envelope.Labels = make([]string, maxSourceLabels+1)
	for index := range envelope.Labels {
		envelope.Labels[index] = "label"
	}
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	_, err = ParseSource(raw)
	require.Error(t, err)
	assert.ErrorContains(t, err, "labels must contain at most")

	candidate := sourceCandidateForTest(t, validSourceEnvelope())
	aliases := make([]string, maxSourceURLAliases+1)
	for index := range aliases {
		aliases[index] = "https://github.com/example/repository/issues/" + jsonNumber(int64(1000+index))
	}
	candidate.URLAliases = aliases
	require.NotNil(t, candidate.Snapshot)
	candidate.Snapshot.URLAliases = append([]string(nil), aliases...)
	err = validateSourceCandidateInput(candidate)
	require.Error(t, err)
	assert.ErrorContains(t, err, "url_aliases must contain at most")
}

func TestImportSourceFileReadsOnceAndDoesNotPersistPath(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(validSourceEnvelope())
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "source.json")
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	pinned, err := ImportSourceFile(path)
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))
	encoded, err := json.Marshal(pinned)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), path)
	assert.NotContains(t, string(encoded), "markdown")
	assert.NotContains(t, string(encoded), "Deliver **value**")
}

func TestImportSourceFileRejectsSymlink(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(validSourceEnvelope())
	require.NoError(t, err)
	directory := t.TempDir()
	target := filepath.Join(directory, "source.json")
	require.NoError(t, os.WriteFile(target, raw, 0o600))
	link := filepath.Join(directory, "source-link.json")
	if err := os.Symlink(filepath.Base(target), link); err != nil {
		t.Skipf("create source symlink: %v", err)
	}

	_, err = ImportSourceFile(link)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
}

func TestParseSourceCandidateAllowsEmptyCommentsForInvalidHead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		mutate             func(*RawSourceEnvelope)
		classificationCode string
	}{
		{
			name: "objective marker has no referenced comments",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Body = strings.Replace(
					envelope.Body,
					changeSourceMarker,
					"<!-- slipway-level: objective/v1 -->",
					1,
				)
				envelope.Comments = []RawSourceComment{}
			},
			classificationCode: SourceClassificationObjectiveMarker,
		},
		{
			name: "manifest references missing comments",
			mutate: func(envelope *RawSourceEnvelope) {
				envelope.Comments = []RawSourceComment{}
			},
			classificationCode: SourceClassificationSectionMissing,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			envelope := validSourceEnvelope()
			test.mutate(&envelope)
			raw, err := json.Marshal(envelope)
			require.NoError(t, err)

			candidate, bodyErr, err := parseSourceCandidate(raw)
			require.NoError(t, err)
			require.Error(t, bodyErr)
			assert.False(t, candidate.Valid)
			assert.Equal(t, test.classificationCode, candidate.ClassificationCode)
			assert.Empty(t, candidate.SourceRevision)
			assert.True(t, validSHA256(candidate.ObservationSHA256))
			assert.Nil(t, candidate.Snapshot)
		})
	}
}

func TestParseSourceCandidateClassifiesBundleFailuresWithoutPersistingRawData(t *testing.T) {
	t.Parallel()
	envelope := validSourceEnvelope()
	envelope.Comments[0].IsMinimized = true
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	candidate, bodyErr, err := parseSourceCandidate(raw)
	require.NoError(t, err)
	require.Error(t, bodyErr)
	assert.False(t, candidate.Valid)
	assert.Equal(t, SourceClassificationSectionMinimized, candidate.ClassificationCode)
	assert.Nil(t, candidate.Snapshot)
	encoded, err := json.Marshal(candidate)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "Deliver **value**")
}

func FuzzParseSourceDeterministic(f *testing.F) {
	valid, err := json.Marshal(validSourceEnvelope())
	if err != nil {
		f.Fatal(err)
	}
	duplicateKey := bytes.Replace(
		valid,
		[]byte(`"source_version":2`),
		[]byte(`"source_version":2,"source_version":2`),
		1,
	)
	controlEnvelope := validSourceEnvelope()
	controlEnvelope.Title += "\u007f"
	control, err := json.Marshal(controlEnvelope)
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range [][]byte{
		valid,
		[]byte(`{`),
		duplicateKey,
		control,
		append([]byte(`{"source_version":2,"title":"`), 0xff, '"', '}'),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		first, firstErr := ParseSource(raw)
		if firstErr != nil {
			return
		}
		second, secondErr := ParseSource(raw)
		if secondErr != nil {
			t.Fatalf("same successful input failed on repeat: %v", secondErr)
		}
		firstJSON, err := json.Marshal(first)
		if err != nil {
			t.Fatal(err)
		}
		secondJSON, err := json.Marshal(second)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(firstJSON, secondJSON) {
			t.Fatalf("same source input produced different pinned projections:\nfirst: %s\nsecond: %s", firstJSON, secondJSON)
		}
		if first.SourceVersion != SourceVersion || first.ParserVersion != ParserVersion ||
			first.ManifestVersion != SourceManifestVersion || first.Profile != SourceProfileChangeV2 {
			t.Fatalf("successful parse violated version/profile invariants: %+v", first)
		}
		for name, revision := range map[string]string{
			"source": first.SourceRevision, "manifest": first.ManifestRevision, "requirements": first.RequirementsRevision,
		} {
			if !validSHA256(revision) {
				t.Fatalf("%s revision is invalid: %q", name, revision)
			}
		}
		if err := validateSourceMaterials(first, true); err != nil {
			t.Fatalf("successful parse produced invalid materials: %v", err)
		}
		for _, section := range first.Sections {
			if section.Bytes <= 0 || !validSHA256(section.SectionRevision) || !validSHA256(section.MaterialSHA256) {
				t.Fatalf("successful parse produced invalid section metadata: %+v", section)
			}
		}
	})
}

func FuzzSourceLineEndingsAndFramedRevisions(f *testing.F) {
	f.Add("line one\nline two\n", "payload")
	f.Add("line one\r\nline two\r", "")
	f.Add("", "\u0000\u007f")

	f.Fuzz(func(t *testing.T, first, second string) {
		normalized := normalizeLineEndings(first)
		if strings.ContainsRune(normalized, '\r') {
			t.Fatalf("normalization retained CR: %q", normalized)
		}
		if normalizeLineEndings(normalized) != normalized {
			t.Fatal("line-ending normalization is not idempotent")
		}
		crlf := strings.ReplaceAll(normalized, "\n", "\r\n")
		cr := strings.ReplaceAll(normalized, "\n", "\r")
		if normalizeLineEndings(crlf) != normalized || normalizeLineEndings(cr) != normalized {
			t.Fatal("CRLF/CR/LF forms did not converge")
		}
		wantBodyRevision := commentBodyRevision(normalized)
		if commentBodyRevision(normalizeLineEndings(crlf)) != wantBodyRevision ||
			commentBodyRevision(normalizeLineEndings(cr)) != wantBodyRevision {
			t.Fatal("normalized line-ending variants changed the comment-body revision")
		}
		if framedRevision(first, second) == framedRevision(first+second) {
			t.Fatal("framing did not separate field boundaries")
		}
		if framedRevision("domain-a/v1", first, second) == framedRevision("domain-b/v1", first, second) {
			t.Fatal("framed revision did not separate domains")
		}
	})
}

func sourceSectionKeys(sections []PinnedSourceSection) []string {
	keys := make([]string, len(sections))
	for index, section := range sections {
		keys[index] = section.Key
	}
	return keys
}

func validSourceEnvelope() RawSourceEnvelope {
	issueURL := "https://github.com/signalridge/slipway/issues/42"
	definitions := []struct {
		key     string
		role    SourceSectionRole
		title   string
		payload string
	}{
		{key: "outcome", role: SourceSectionOutcome, title: "Outcome", payload: "\n# Outcome\n\nDeliver **value**.  \n`literal`\n"},
		{key: "requirements", role: SourceSectionRequirements, title: "Requirements", payload: "\n# Requirements\n\n- Keep order.\n- Preserve  spaces.\n"},
		{key: "acceptance-examples", role: SourceSectionAcceptanceExamples, title: "Acceptance examples", payload: "\n# Acceptance examples\n\n```text\n## Requirements\n```\n"},
		{key: "constraints", role: SourceSectionConstraints, title: "Constraints", payload: "\n# Constraints\n\n\tTabbed body text\n"},
		{key: "non-goals", role: SourceSectionNonGoals, title: "Non-goals", payload: "\n# Non-goals\n\nNone.\n"},
	}
	comments := make([]RawSourceComment, len(definitions))
	for index, definition := range definitions {
		databaseID := int64(101 + index)
		comments[index] = RawSourceComment{
			NodeID:     "IC_section_" + definition.key,
			DatabaseID: databaseID,
			URL:        issueURL + "#issuecomment-" + jsonNumber(databaseID),
			UpdatedAt:  "2026-07-12T09:00:00Z",
			AuthorID:   "U_author",
			Body:       sectionMarkerPrefix + definition.key + " -->" + definition.payload,
		}
	}
	envelope := RawSourceEnvelope{
		SourceVersion: SourceVersion,
		Provider:      "github",
		Host:          "github.com",
		RepositoryID:  "R_kgDOExample",
		IssueID:       "I_kwDOExample",
		IssueNumber:   42,
		CanonicalURL:  issueURL,
		UpdatedAt:     "2026-07-12T08:00:00Z",
		FetchedAt:     "2026-07-12T09:01:00Z",
		Title:         "[Change] Preserve source requirements",
		Labels:        []string{"level:change", "kind:refactor"},
		Parent: &SourceParent{
			RepositoryID: "R_kgDOExample",
			IssueID:      "I_kwDOParent",
			CanonicalURL: "https://github.com/signalridge/slipway/issues/40",
		},
		Comments: comments,
	}
	rebuildSourceManifestBody(nil, &envelope)
	return envelope
}

func setEnvelopeSection(t *testing.T, envelope *RawSourceEnvelope, key, payload string) {
	if t != nil {
		t.Helper()
	}
	for index := range envelope.Comments {
		if testSourceCommentKey(envelope.Comments[index].Body) == key {
			envelope.Comments[index].NodeID += "_replacement"
			envelope.Comments[index].DatabaseID += 100_000
			envelope.Comments[index].URL = envelope.CanonicalURL + "#issuecomment-" + jsonNumber(envelope.Comments[index].DatabaseID)
			envelope.Comments[index].Body = sectionMarkerPrefix + key + " -->" + payload
			rebuildSourceManifestBody(t, envelope)
			return
		}
	}
	if t != nil {
		t.Fatalf("section %q not found", key)
	}
	panic("section not found: " + key)
}

func setEnvelopeParentRequirementsRevision(
	t *testing.T,
	envelope *RawSourceEnvelope,
	revision string,
) {
	t.Helper()
	manifest, err := parseSourceManifest(normalizeLineEndings(envelope.Body))
	require.NoError(t, err)
	manifest.ParentRequirementsRevision = revision
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	envelope.Body = changeSourceMarker + "\n\n" + sourceManifestFence + "\n" + string(encoded) + "\n```\n"
}

func testSourceCommentKey(body string) string {
	lines := strings.Split(normalizeLineEndings(body), "\n")
	index := firstNonemptyLine(lines, 0)
	if index < 0 {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(lines[index], sectionMarkerPrefix), " -->")
}

func rebuildSourceManifestBody(t *testing.T, envelope *RawSourceEnvelope) {
	if t != nil {
		t.Helper()
	}
	roles := map[string]SourceSectionRole{
		"outcome":             SourceSectionOutcome,
		"requirements":        SourceSectionRequirements,
		"acceptance-examples": SourceSectionAcceptanceExamples,
		"constraints":         SourceSectionConstraints,
		"non-goals":           SourceSectionNonGoals,
	}
	titles := map[string]string{
		"outcome":             "Outcome",
		"requirements":        "Requirements",
		"acceptance-examples": "Acceptance examples",
		"constraints":         "Constraints",
		"non-goals":           "Non-goals",
	}
	sections := make([]SourceManifestSection, len(envelope.Comments))
	for index, comment := range envelope.Comments {
		key := testSourceCommentKey(comment.Body)
		sections[index] = SourceManifestSection{
			Key:               key,
			Role:              roles[key],
			Title:             titles[key],
			CommentNodeID:     comment.NodeID,
			CommentDatabaseID: comment.DatabaseID,
			BodySHA256:        commentBodyRevision(normalizeLineEndings(comment.Body)),
		}
	}
	manifest := SourceManifest{
		ManifestVersion: SourceManifestVersion,
		Profile:         SourceProfileChangeV2,
		Sections:        sections,
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		if t != nil {
			t.Fatal(err)
		}
		panic(err)
	}
	envelope.Body = changeSourceMarker + "\n\n" + sourceManifestFence + "\n" + string(encoded) + "\n```\n"
}

func mustParseSource(t *testing.T, envelope RawSourceEnvelope) PinnedSource {
	t.Helper()
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	pinned, err := ParseSource(raw)
	require.NoError(t, err)
	return pinned
}

func jsonNumber(value int64) string {
	return strings.TrimSpace(string(mustJSON(value)))
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}
