// Bounded Action context assembly. Context is a projection of confirmed
// decisions and prior observations that must fit the Action byte limit
// deterministically; Requirements are authority and are never truncated here.

package autopilot

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

func actionContextEncodedLimit(action Action) (int, error) {
	action.Context = ""
	encoded, err := encodeAction(action)
	if err != nil {
		return 0, fmt.Errorf("encode action without context: %w", err)
	}
	emptyStringSize, err := encodedJSONStringSize("")
	if err != nil {
		return 0, fmt.Errorf("encode empty action context: %w", err)
	}
	limit := maxActionBytes - len(encoded) + emptyStringSize
	if limit < emptyStringSize {
		return 0, fmt.Errorf("%w %d bytes", errActionTooLarge, maxActionBytes)
	}
	return limit, nil
}

func truncateUTF8WithMarker(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	marker := contextTruncationMarker(value)
	if len(marker) >= limit {
		return marker[:limit]
	}
	prefix := limit - len(marker)
	for prefix > 0 && !utf8.ValidString(value[:prefix]) {
		prefix--
	}
	return value[:prefix] + marker
}

type contextCandidate struct {
	class    *contextClass
	content  string
	selected string
}

type contextClass struct {
	heading    string
	omittedKey string
	candidates []*contextCandidate
}

func buildContext(run Run) (string, error) {
	return buildContextWithinEncodedLimit(run, int(^uint(0)>>1))
}

func buildContextWithinEncodedLimit(run Run, maxEncodedBytes int) (string, error) {
	decisions := &contextClass{heading: "Decisions:", omittedKey: "decisions", candidates: make([]*contextCandidate, 0)}
	recent := &contextClass{heading: "Recent outcome:", omittedKey: "recent outcomes", candidates: make([]*contextCandidate, 0, 1)}
	earlier := &contextClass{heading: "Earlier outcomes:", omittedKey: "earlier outcomes", candidates: make([]*contextCandidate, 0)}
	classes := []*contextClass{decisions, recent, earlier}

	for _, answer := range run.Answers {
		if !answer.Active || answer.Text == "" || answer.ConfirmDestructive {
			continue
		}
		if run.PinnedSource == nil {
			if answer.RequirementsRevision != "" {
				continue
			}
		} else if answer.RequirementsRevision != run.PinnedSource.RequirementsRevision {
			continue
		}
		item := fmt.Sprintf("- action %s decision:\n%s\n", answer.ActionID, indentContextText(answer.Text, "  "))
		normalized, err := normalizeContextItem(item)
		if err != nil {
			return "", fmt.Errorf("normalize decision context: %w", err)
		}
		candidate := &contextCandidate{class: decisions, content: normalized}
		decisions.candidates = append(decisions.candidates, candidate)
	}

	outcomes := make([]*contextCandidate, 0, len(run.Actions))
	for _, record := range run.Actions {
		if record.Outcome == nil || record.Voided {
			continue
		}
		item := renderOutcomeContextItem(record)
		normalized, err := normalizeContextItem(item)
		if err != nil {
			return "", fmt.Errorf("normalize outcome context: %w", err)
		}
		outcomes = append(outcomes, &contextCandidate{content: normalized})
	}
	if len(outcomes) > 0 {
		latest := outcomes[len(outcomes)-1]
		latest.class = recent
		recent.candidates = append(recent.candidates, latest)
		for _, candidate := range outcomes[:len(outcomes)-1] {
			candidate.class = earlier
			earlier.candidates = append(earlier.candidates, candidate)
		}
	}

	priority := make([]*contextCandidate, 0, len(decisions.candidates)+len(outcomes))
	for index := len(decisions.candidates) - 1; index >= 0; index-- {
		priority = append(priority, decisions.candidates[index])
	}
	if len(recent.candidates) == 1 {
		priority = append(priority, recent.candidates[0])
	}
	for index := len(earlier.candidates) - 1; index >= 0; index-- {
		priority = append(priority, earlier.candidates[index])
	}

	for _, candidate := range priority {
		candidate.selected = candidate.content
		fits, err := renderedContextFits(classes, maxEncodedBytes)
		if err != nil {
			return "", err
		}
		if fits {
			continue
		}
		candidate.selected = ""
		marker := contextTruncationMarker(candidate.content)
		candidate.selected = marker + "\n"
		fits, err = renderedContextFits(classes, maxEncodedBytes)
		if err != nil {
			return "", err
		}
		if !fits {
			candidate.selected = ""
			break
		}
		prefix, err := maxFittingContextPrefix(candidate, classes, marker, maxEncodedBytes)
		if err != nil {
			return "", err
		}
		candidate.selected = candidate.content[:prefix] + marker + "\n"
		break
	}

	context := renderContext(classes)
	if len(context) > maxActionContextBytes {
		return "", fmt.Errorf("context exceeds %d bytes", maxActionContextBytes)
	}
	if !utf8.ValidString(context) {
		return "", errors.New("context must be valid utf-8")
	}
	encodedSize, err := encodedJSONStringSize(context)
	if err != nil {
		return "", fmt.Errorf("encode action context: %w", err)
	}
	if encodedSize > maxEncodedBytes {
		return "", fmt.Errorf("%w %d bytes after bounded context projection", errActionTooLarge, maxActionBytes)
	}
	return context, nil
}

func renderedContextFits(classes []*contextClass, maxEncodedBytes int) (bool, error) {
	context := renderContext(classes)
	if len(context) > maxActionContextBytes {
		return false, nil
	}
	encodedSize, err := encodedJSONStringSize(context)
	if err != nil {
		return false, fmt.Errorf("encode action context: %w", err)
	}
	return encodedSize <= maxEncodedBytes, nil
}

func maxFittingContextPrefix(candidate *contextCandidate, classes []*contextClass, marker string, maxEncodedBytes int) (int, error) {
	limit := min(len(candidate.content), maxActionContextBytes)
	boundaries := make([]int, 1, limit+1)
	for index := range candidate.content {
		if index == 0 {
			continue
		}
		if index > limit {
			break
		}
		boundaries = append(boundaries, index)
	}
	if len(candidate.content) <= limit && boundaries[len(boundaries)-1] != len(candidate.content) {
		boundaries = append(boundaries, len(candidate.content))
	}

	var fitErr error
	firstTooLarge := sort.Search(len(boundaries), func(index int) bool {
		candidate.selected = candidate.content[:boundaries[index]] + marker + "\n"
		fits, err := renderedContextFits(classes, maxEncodedBytes)
		if err != nil {
			fitErr = err
			return true
		}
		return !fits
	})
	if fitErr != nil {
		return 0, fitErr
	}
	if firstTooLarge == 0 {
		return 0, nil
	}
	if firstTooLarge == len(boundaries) {
		return boundaries[len(boundaries)-1], nil
	}
	return boundaries[firstTooLarge-1], nil
}

func renderOutcomeContextItem(record ActionRecord) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "- %s action %s: %s\n", record.Action.Kind, record.Action.ActionID, record.Outcome.Summary)
	if len(record.Outcome.KnownIssues) > 0 {
		builder.WriteString("  Known issues:\n")
		for _, issue := range record.Outcome.KnownIssues {
			builder.WriteString("  - ")
			builder.WriteString(indentContextContinuation(issue, "    "))
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func normalizeContextItem(value string) (string, error) {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	if !utf8.ValidString(value) {
		return "", errors.New("context candidate must be valid utf-8")
	}
	return value, nil
}

func indentContextText(value, indentation string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return indentation + strings.ReplaceAll(value, "\n", "\n"+indentation)
}

func indentContextContinuation(value, indentation string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.ReplaceAll(value, "\n", "\n"+indentation)
}

func contextTruncationMarker(normalized string) string {
	digest := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("...[truncated original_bytes=%d sha256=%x]", len(normalized), digest)
}

func renderContext(classes []*contextClass) string {
	var builder strings.Builder
	for _, class := range classes {
		builder.WriteString(class.heading)
		builder.WriteByte('\n')
		selected := 0
		for _, candidate := range class.candidates {
			if candidate.selected == "" {
				continue
			}
			builder.WriteString(candidate.selected)
			selected++
		}
		if len(class.candidates) == 0 {
			builder.WriteString("(none)\n")
		} else if omitted := len(class.candidates) - selected; omitted > 0 {
			fmt.Fprintf(&builder, "[omitted %s: %d]\n", class.omittedKey, omitted)
		}
	}
	return builder.String()
}
