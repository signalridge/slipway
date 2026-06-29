package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
)

const (
	handoffTitle       = "# Slipway Runtime Session Handoff"
	handoffHeaderOpen  = "<!-- slipway:handoff-machine-header"
	handoffHeaderClose = "slipway:handoff-machine-header -->"
	handoffTimeFormat  = time.RFC3339Nano
	// handoffPendingMarker is the placeholder body written for any advisory
	// section that has no agent-authored narrative yet.
	handoffPendingMarker = "_Agent-authored narrative pending._"
	// handoffDefaultSection receives a free-form handoff body that does not carry
	// any recognizable `## Section` headers, so piped narrative is never dropped.
	handoffDefaultSection = "Current Position"
)

var handoffSectionNames = []string{
	"Current Position",
	"Session Work Completed",
	"Next Session Focus",
	"Path References",
	"Suggested Next Skills",
	"Risks And Blockers",
	"Redaction Check",
}

// HandoffHeader is the engine-owned machine descriptor for a per-change
// runtime handoff. It intentionally carries identity and freshness only; the
// lifecycle position and next action remain owned by status/next.
type HandoffHeader struct {
	Slug         string    `json:"slug"`
	Generation   int       `json:"generation"`
	SessionOwner string    `json:"session_owner"`
	GitBranch    string    `json:"git_branch"`
	Worktree     string    `json:"worktree"`
	UpdatedAt    time.Time `json:"updated_at"`
	Staleness    string    `json:"staleness"`
}

type HandoffDocument struct {
	Path      string
	Header    HandoffHeader
	Narrative string
}

type HandoffWriteOptions struct {
	Now          time.Time
	SessionOwner string
	Section      string
	SectionBody  string
	// Body is a full advisory narrative (typically piped on stdin for the bare
	// `slipway handoff write` form). Sections recognized inside the body are
	// merged over the existing narrative; a body with no recognizable section
	// headers is routed into handoffDefaultSection so nothing is dropped.
	Body string
}

func WriteHandoff(root string, change model.Change, opts HandoffWriteOptions) (HandoffDocument, error) {
	if err := ValidateChangeSlug(change.Slug); err != nil {
		return HandoffDocument{}, fmt.Errorf("invalid handoff change slug %q: %w", change.Slug, err)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	path := ChangeHandoffPath(root, change.Slug)
	existing, _ := ReadHandoffFile(path)
	narrative := ensureHandoffNarrativeSkeleton(existing.Narrative)
	if body := strings.TrimSpace(opts.Body); body != "" {
		narrative = mergeHandoffBody(narrative, body)
	}
	if section := strings.TrimSpace(opts.Section); section != "" && strings.TrimSpace(opts.SectionBody) != "" {
		narrative = replaceHandoffSection(narrative, section, strings.TrimSpace(opts.SectionBody))
	}

	header, err := buildHandoffHeader(root, change, now, existing.Header.Generation+1, opts.SessionOwner)
	if err != nil {
		return HandoffDocument{}, err
	}
	doc := HandoffDocument{
		Path:      path,
		Header:    header,
		Narrative: narrative,
	}
	if err := fsutil.WriteFileAtomic(path, []byte(RenderHandoff(doc)), 0o644); err != nil {
		return HandoffDocument{}, err
	}
	return doc, nil
}

func ReadHandoff(root string, change model.Change) (HandoffDocument, error) {
	if err := ValidateChangeSlug(change.Slug); err != nil {
		return HandoffDocument{}, fmt.Errorf("invalid handoff change slug %q: %w", change.Slug, err)
	}
	path := ChangeHandoffPath(root, change.Slug)
	doc, err := ReadHandoffFile(path)
	if err != nil {
		return HandoffDocument{}, err
	}
	doc.Path = path
	doc.Header.Staleness = HandoffStaleness(root, change, doc.Header.UpdatedAt)
	doc.Narrative = ensureHandoffNarrativeSkeleton(doc.Narrative)
	return doc, nil
}

func ReadHandoffFile(path string) (HandoffDocument, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- path is a Slipway runtime path resolved by state helpers.
	if err != nil {
		return HandoffDocument{Path: path}, err
	}
	return ParseHandoff(path, string(raw))
}

func ParseHandoff(path, raw string) (HandoffDocument, error) {
	doc := HandoffDocument{Path: path}
	start := strings.Index(raw, handoffHeaderOpen)
	end := strings.Index(raw, handoffHeaderClose)
	if start >= 0 && end > start {
		headerStart := start + len(handoffHeaderOpen)
		headerRaw := strings.TrimSpace(raw[headerStart:end])
		if err := json.Unmarshal([]byte(headerRaw), &doc.Header); err != nil {
			return HandoffDocument{}, fmt.Errorf("parse handoff machine header: %w", err)
		}
		doc.Narrative = strings.TrimSpace(raw[end+len(handoffHeaderClose):])
		return doc, nil
	}
	doc.Narrative = strings.TrimSpace(raw)
	return doc, nil
}

func RenderHandoff(doc HandoffDocument) string {
	headerRaw, _ := json.MarshalIndent(doc.Header, "", "  ")
	var b strings.Builder
	b.WriteString(handoffTitle)
	b.WriteString("\n\n")
	b.WriteString(handoffHeaderOpen)
	b.WriteByte('\n')
	b.Write(headerRaw)
	b.WriteByte('\n')
	b.WriteString(handoffHeaderClose)
	b.WriteString("\n\n")
	b.WriteString(ensureHandoffNarrativeSkeleton(doc.Narrative))
	b.WriteString("\n")
	return b.String()
}

func HandoffBrief(doc HandoffDocument) string {
	if strings.TrimSpace(doc.Header.Slug) == "" {
		return ""
	}
	return fmt.Sprintf(
		"session_handoff: slug=%s generation=%d updated_at=%s session_owner=%s staleness=%s path=%s focus=%s",
		doc.Header.Slug,
		doc.Header.Generation,
		doc.Header.UpdatedAt.UTC().Format(handoffTimeFormat),
		doc.Header.SessionOwner,
		doc.Header.Staleness,
		doc.Path,
		handoffOneLineFocus(doc.Narrative),
	)
}

func HandoffStaleness(root string, change model.Change, updatedAt time.Time) string {
	if updatedAt.IsZero() {
		return "unknown"
	}
	events, err := ReadLifecycleEvents(root, change)
	if err != nil || len(events) == 0 {
		return "fresh"
	}
	latest := events[0].OccurredAt
	for _, event := range events[1:] {
		if event.OccurredAt.After(latest) {
			latest = event.OccurredAt
		}
	}
	if latest.After(updatedAt.UTC()) {
		return "stale"
	}
	return "fresh"
}

func buildHandoffHeader(root string, change model.Change, now time.Time, generation int, sessionOwner string) (HandoffHeader, error) {
	workspaceRoot, err := WorkspaceRootForChange(root, change)
	if err != nil {
		return HandoffHeader{}, err
	}
	if generation <= 0 {
		generation = 1
	}
	if sessionOwner = strings.TrimSpace(sessionOwner); sessionOwner == "" {
		sessionOwner = defaultHandoffSessionOwner()
	}
	branch := "unknown"
	if value, ok := gitBranchFromMetadata(workspaceRoot); ok && strings.TrimSpace(value) != "" {
		branch = value
	}
	return HandoffHeader{
		Slug:         change.Slug,
		Generation:   generation,
		SessionOwner: sessionOwner,
		GitBranch:    branch,
		Worktree:     DisplayPath(root, workspaceRoot),
		UpdatedAt:    now.UTC(),
		Staleness:    HandoffStaleness(root, change, now),
	}, nil
}

func defaultHandoffSessionOwner() string {
	for _, key := range []string{"SLIPWAY_SESSION_OWNER", "USER", "USERNAME"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	if host, err := os.Hostname(); err == nil && strings.TrimSpace(host) != "" {
		return strings.TrimSpace(host)
	}
	return "unknown"
}

func ensureHandoffNarrativeSkeleton(raw string) string {
	raw = strings.TrimSpace(removeHandoffTitle(raw))
	var b strings.Builder
	b.WriteString("This handoff is advisory only. Run `slipway status --json` and `slipway next --json` for lifecycle authority before acting.\n")
	for _, section := range handoffSectionNames {
		body := extractHandoffSection(raw, section)
		if strings.TrimSpace(body) == "" {
			body = handoffPendingMarker
		}
		b.WriteString("\n## ")
		b.WriteString(section)
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(body))
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func removeHandoffTitle(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, handoffTitle)
	return strings.TrimSpace(raw)
}

func extractHandoffSection(raw, section string) string {
	re := regexp.MustCompile(`(?ms)^## ` + regexp.QuoteMeta(section) + `\s*\n(.*?)(?:\n## |\z)`)
	match := re.FindStringSubmatch(raw)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func replaceHandoffSection(raw, section, body string) string {
	canonical := canonicalHandoffSection(section)
	if canonical == "" {
		canonical = section
	}
	raw = ensureHandoffNarrativeSkeleton(raw)
	heading := "## " + canonical + "\n"
	start := strings.Index(raw, heading)
	replacement := heading + strings.TrimSpace(body)
	if start < 0 {
		return strings.TrimSpace(raw) + "\n\n" + replacement
	}
	bodyStart := start + len(heading)
	end := len(raw)
	if next := strings.Index(raw[bodyStart:], "\n## "); next >= 0 {
		end = bodyStart + next
	}
	return strings.TrimSpace(raw[:bodyStart]) + "\n" + strings.TrimSpace(body) + "\n" + strings.TrimLeft(raw[end:], "\n")
}

func canonicalHandoffSection(section string) string {
	normalized := normalizeSectionName(section)
	for _, candidate := range handoffSectionNames {
		if normalizeSectionName(candidate) == normalized {
			return candidate
		}
	}
	return ""
}

// HandoffSectionNames returns the canonical advisory handoff section names in
// document order. Command surfaces use it to validate `--section` input and to
// list valid sections in guidance.
func HandoffSectionNames() []string {
	return slices.Clone(handoffSectionNames)
}

// CanonicalHandoffSection resolves a user-supplied section name to its canonical
// form. ok is false when the name does not match a known advisory section, so
// callers can fail loudly instead of writing a non-canonical section.
func CanonicalHandoffSection(name string) (string, bool) {
	canonical := canonicalHandoffSection(name)
	if canonical == "" {
		return "", false
	}
	return canonical, true
}

// HandoffIsEmpty reports whether every advisory section is still the pending
// placeholder, i.e. no agent narrative has been recorded yet. Read surfaces use
// it to emit a clear "empty / all sections pending" notice instead of rendering
// a content-free scaffold as if it were a real handoff.
func HandoffIsEmpty(doc HandoffDocument) bool {
	narrative := ensureHandoffNarrativeSkeleton(doc.Narrative)
	for _, section := range handoffSectionNames {
		body := strings.TrimSpace(extractHandoffSection(narrative, section))
		if body != "" && body != handoffPendingMarker {
			return false
		}
	}
	return true
}

// mergeHandoffBody overlays a full advisory narrative onto the existing one.
// Sections recognized inside body replace their counterparts; a body with no
// recognizable `## Section` headers is routed into handoffDefaultSection so a
// piped narrative is never silently dropped.
func mergeHandoffBody(existing, body string) string {
	merged := ensureHandoffNarrativeSkeleton(existing)
	body = strings.TrimSpace(body)
	if body == "" {
		return merged
	}
	matched := false
	for _, section := range handoffSectionNames {
		sectionBody := strings.TrimSpace(extractHandoffSection(body, section))
		if sectionBody == "" {
			continue
		}
		merged = replaceHandoffSection(merged, section, sectionBody)
		matched = true
	}
	if !matched {
		merged = replaceHandoffSection(merged, handoffDefaultSection, body)
	}
	return merged
}

func normalizeSectionName(section string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(section, "-", " "))), " ")
}

func handoffOneLineFocus(narrative string) string {
	for _, section := range []string{"Next Session Focus", "Current Position"} {
		body := extractHandoffSection(narrative, section)
		for _, line := range strings.Split(body, "\n") {
			line = strings.Trim(strings.TrimSpace(line), "-* ")
			if line != "" && !strings.Contains(line, "pending") {
				return compactHandoffLine(line)
			}
		}
	}
	return "run slipway handoff show for narrative context"
}

func compactHandoffLine(line string) string {
	line = strings.Join(strings.Fields(line), " ")
	if len(line) <= 120 {
		return line
	}
	return strings.TrimSpace(line[:117]) + "..."
}

func HandoffPathForDisplay(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func HandoffHeaderKeys() []string {
	keys := []string{"slug", "generation", "session_owner", "git_branch", "worktree", "updated_at", "staleness"}
	slices.Sort(keys)
	return keys
}
