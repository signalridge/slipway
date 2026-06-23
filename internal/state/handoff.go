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

	// handoffPendingPlaceholder is the body the engine writes for every section
	// of a freshly scaffolded handoff. A section still carrying this exact body
	// is unauthored and must not be surfaced as continuity content.
	handoffPendingPlaceholder = "_Agent-authored narrative pending._"

	// handoffExcerptSectionMaxRunes bounds each section body surfaced in the
	// SessionStart excerpt so the host context budget stays protected. The fixed
	// handoff template means the excerpt is deterministic; only oversized authored
	// bodies are truncated, with a pointer to the full narrative.
	handoffExcerptSectionMaxRunes = 400
)

// handoffExcerptSections is the fixed, ordered set of continuity sections
// surfaced in the SessionStart excerpt. The handoff template is fixed, so the
// excerpt always pulls the same well-known sections when authored.
var handoffExcerptSections = []string{
	"Current Position",
	"Next Session Focus",
	"Risks And Blockers",
}

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

// HandoffExcerpt renders a bounded, host-injectable view of a handoff for
// SessionStart context. It leads with the machine brief line, then appends the
// authored continuity sections from the fixed handoff template. Unauthored
// placeholder bodies are skipped; when no section is authored the excerpt
// degrades to the brief plus an explicit unauthored marker so the host knows
// the handoff exists but carries no continuity yet. A stale handoff is flagged
// so the host re-authors before relying on it. Returns "" when the document
// carries no slug.
func HandoffExcerpt(doc HandoffDocument) string {
	brief := HandoffBrief(doc)
	if brief == "" {
		return ""
	}
	var b strings.Builder
	if strings.EqualFold(strings.TrimSpace(doc.Header.Staleness), "stale") {
		b.WriteString("session_handoff_stale: true; lifecycle advanced after this handoff — re-author via `slipway handoff write` before relying on it\n")
	}
	b.WriteString(brief)
	b.WriteByte('\n')

	sections := authoredHandoffSections(doc.Narrative)
	if len(sections) == 0 {
		b.WriteString("session_handoff_unauthored: true; no continuity recorded — run `slipway handoff write --section \"Next Session Focus\"` to capture it")
		return strings.TrimRight(b.String(), "\n")
	}
	b.WriteString("session_handoff_excerpt:\n")
	for _, section := range sections {
		b.WriteString("## ")
		b.WriteString(section[0])
		b.WriteByte('\n')
		b.WriteString(truncateHandoffBody(section[1], handoffExcerptSectionMaxRunes))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// authoredHandoffSections returns the fixed-template continuity sections that
// carry real agent-authored content, in template order, skipping any section
// whose body is still the engine placeholder.
func authoredHandoffSections(narrative string) [][2]string {
	out := make([][2]string, 0, len(handoffExcerptSections))
	for _, section := range handoffExcerptSections {
		body := strings.TrimSpace(extractHandoffSection(narrative, section))
		if !handoffSectionAuthored(body) {
			continue
		}
		out = append(out, [2]string{section, body})
	}
	return out
}

// handoffSectionAuthored reports whether a section body carries real content
// rather than the engine's unauthored placeholder.
func handoffSectionAuthored(body string) bool {
	body = strings.TrimSpace(body)
	return body != "" && body != handoffPendingPlaceholder
}

// truncateHandoffBody bounds a section body to maxRunes, appending a pointer to
// the full narrative when truncated. Truncation is rune-aware so multibyte
// content is never split mid-rune.
func truncateHandoffBody(body string, maxRunes int) string {
	body = strings.TrimSpace(body)
	runes := []rune(body)
	if len(runes) <= maxRunes {
		return body
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + " …(run `slipway handoff show` for full text)"
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
			body = handoffPendingPlaceholder
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
