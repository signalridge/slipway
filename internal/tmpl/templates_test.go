package tmpl

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedSurfaceContainsOnlySixCapabilitiesAndOneReference(t *testing.T) {
	t.Parallel()
	var files []string
	err := fs.WalkDir(TemplateFS(), "skills", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(files)
	assert.Equal(t, []string{
		"skills/clarify/SKILL.md",
		"skills/clarify/references/decision-interview.md",
		"skills/decompose/SKILL.md",
		"skills/implement/SKILL.md",
		"skills/propose/SKILL.md",
		"skills/review/SKILL.md",
		"skills/run/SKILL.md",
	}, files)
}

func TestCapabilityPromptsEncodeAcceptanceBehavior(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     string
		contains []string
	}{
		{path: "skills/run/SKILL.md", contains: []string{"explicitly asks", "marker-valid Run", "structured `next.variants`", "complete pinned", "`grill-me` design-tree discipline", "shared understanding", "Strict Outcome shape", "`action_kind` is mandatory", "`skipped` is emitted only by the CLI", "Review never needs input", "Every Action may be skipped", "automatic Action queue is empty", "`gh >= 2.94.0`", "official REST fallback", "redirects/transfers only within `github.com`", "accepted five Requirements sections", "Redact recognized credentials"}},
		{path: "skills/clarify/SKILL.md", contains: []string{"explicitly invokes", "design-tree discipline", "exactly one decision", "recommended option", "ask zero questions", "shared understanding", "wrap up", "Do not implement", "stateless", "`grill-with-docs`"}},
		{path: "skills/propose/SKILL.md", contains: []string{"explicitly asks", "self-contained", "exactly one `level:change`", "exactly one `level:objective`", "exactly one `kind:*`", "marker remains authority", "exactly three choices", "`gh >= 2.94.0`", "official GitHub REST API", "100 sub-issues", "50 blocking", "same-host redirect or transfer", "approved publication plan", "operation UUID", "expected current revisions", "body files", "timeout-after-success", "indexing delay", "`created`, `matched`, `failed`, or `ambiguous`", "Zero matches", "public repository has no per-Issue private switch", "Redact recognized credential values", "environment_unavailable"}},
		{path: "skills/decompose/SKILL.md", contains: []string{"explicitly asks", "tracer-bullet Changes", "runtime inheritance", "exactly one `level:objective`", "exactly one `level:change`", "100 sub-issues", "50 dependencies", "`gh >= 2.94.0`", "official REST API", "cross-host redirects", "approved publication plan", "stable item UUIDs", "read back", "duplicate marker matches", "Zero marker matches", "public Issue has no private switch", "amendment mode", "Never propagate in the background"}},
		{path: "skills/implement/SKILL.md", contains: []string{"explicitly invokes", "pinned Requirements", "`action_kind: \"implement\"`", "activities` (which may be empty)", "actual positive attempt count", "exact command", "exit code", "Never list an activity that did not run", "shell exit 127", "scope SHA-256", "directly outside a Run"}},
		{path: "skills/review/SKILL.md", contains: []string{"explicitly invokes", "always read-only", "Intent", "Quality", "start-to-current difference is only an observation", "`action_kind: \"review\"`", "findings_reported", "leave `suggested_actions` empty", "Do not modify files", "automatic repair or re-review loop"}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()
			content, err := Content(test.path)
			require.NoError(t, err)
			for _, fragment := range test.contains {
				assert.Contains(t, content, fragment)
			}
		})
	}
}

func TestSharedCapabilityBoundariesEncodeTrustAndUserControl(t *testing.T) {
	t.Parallel()
	content, err := Content("_partials/common.tmpl")
	require.NoError(t, err)
	for _, fragment := range []string{
		"untrusted data",
		"exact first body marker is Level authority",
		"trusted attester",
		"Never invent a local Issue",
		"accepted Requirements, user answers, goals, and truthful command summaries may contain sensitive text",
		"public-repository Issue has no private switch",
		"Redact recognized credential values",
		"preserving truthful command identity",
		"tokens, raw comments, environment dumps, full transcripts, or hidden reasoning",
		"exact draft and operation plan",
		"Never blindly retry",
		"Natural-language approval alone is not a grant",
	} {
		assert.Contains(t, content, fragment)
	}
}

func TestGeneratedTemplatesAvoidRetiredRuntimeVocabularyAndReviewRatings(t *testing.T) {
	t.Parallel()
	retired := regexp.MustCompile(`(?i)\b(?:gate|done[_-]?ready|ship[_-]?ready|lifecycle)\b`)
	err := fs.WalkDir(TemplateFS(), "skills", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := fs.ReadFile(TemplateFS(), path)
		if err != nil {
			return err
		}
		assert.Empty(t, retired.FindString(string(content)), path)
		return nil
	})
	require.NoError(t, err)

	review, err := Content("skills/review/SKILL.md")
	require.NoError(t, err)
	for _, rating := range []string{" pass ", " fail ", " approved ", " ready ", " verdict "} {
		assert.NotContains(t, " "+strings.ToLower(review)+" ", rating)
	}
}

func TestDecisionInterviewReferenceRetainsMITAttribution(t *testing.T) {
	t.Parallel()
	content, err := Content("skills/clarify/references/decision-interview.md")
	require.NoError(t, err)
	assert.Contains(t, content, "mattpocock/skills")
	assert.Contains(t, content, "MIT License")
	assert.Contains(t, content, "Copyright (c) 2026 Matt Pocock")
	assert.Contains(t, content, "permission notice")
}
