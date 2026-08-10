// Final Run summary rendering. The summary reports what was observed and what
// was not run, within a bounded size, and certifies nothing.

package autopilot

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxFinalSummaryBytes = 64 << 10

type boundedSummaryBuilder struct {
	limit  int
	prefix []byte
	total  int
	hash   hash.Hash
}

func newBoundedSummaryBuilder(limit int) *boundedSummaryBuilder {
	return &boundedSummaryBuilder{
		limit:  limit,
		prefix: make([]byte, 0, limit),
		hash:   sha256.New(),
	}
}

func (builder *boundedSummaryBuilder) Write(data []byte) (int, error) {
	if builder == nil {
		return 0, errors.New("write summary: nil builder")
	}
	_, _ = builder.hash.Write(data)
	builder.total += len(data)
	if remaining := builder.limit - len(builder.prefix); remaining > 0 {
		if len(data) < remaining {
			remaining = len(data)
		}
		builder.prefix = append(builder.prefix, data[:remaining]...)
	}
	return len(data), nil
}

func (builder *boundedSummaryBuilder) WriteString(value string) {
	_, _ = builder.Write([]byte(value))
}

func (builder *boundedSummaryBuilder) String() string {
	if builder.total <= builder.limit {
		return string(builder.prefix)
	}
	digest := fmt.Sprintf("sha256:%x", builder.hash.Sum(nil))
	cut := builder.limit
	marker := ""
	for range 4 {
		omitted := builder.total - cut
		marker = fmt.Sprintf("\n\n...[summary truncated omitted_bytes=%d original_sha256=%s]\n", omitted, digest)
		cut = builder.limit - len(marker)
		if cut < 0 {
			cut = 0
		}
		if cut > len(builder.prefix) {
			cut = len(builder.prefix)
		}
		for cut > 0 && !utf8.Valid(builder.prefix[:cut]) {
			cut--
		}
	}
	return string(builder.prefix[:cut]) + marker
}

func writeSummaryStrings(builder *boundedSummaryBuilder, heading string, values []string) {
	if len(values) == 0 {
		return
	}
	builder.WriteString(heading + "\n")
	for _, value := range values {
		fmt.Fprintf(builder, "- %s\n", value)
	}
}

func finalSummary(run Run) string {
	builder := newBoundedSummaryBuilder(maxFinalSummaryBytes)
	builder.WriteString("The automatic action queue has ended.\n")
	builder.WriteString("Observed action reports:\n")

	changedFiles := map[string]struct{}{}
	var confirmedDecisions []AnswerRecord
	var observations []string
	var reviewFindings []Finding
	var skipped, voided []string
	for _, answer := range run.Answers {
		if answer.Active && answer.SupersededBy == "" {
			confirmedDecisions = append(confirmedDecisions, answer)
		}
	}
	var reviewOutcome *Outcome
	var reviewSkippedByUser, reviewVoided bool
	for index := range run.Actions {
		record := &run.Actions[index]
		if record.Skipped {
			skipped = append(skipped, string(record.Action.Kind))
			if record.Action.Kind == ActionReview {
				reviewSkippedByUser = true
			}
		}
		if record.Voided {
			voided = append(voided, string(record.Action.Kind))
			if record.Action.Kind == ActionReview {
				reviewVoided = true
			}
		}
		if record.Outcome == nil {
			continue
		}
		annotation := ""
		if record.Skipped {
			annotation = " (skipped)"
		} else if record.Voided {
			annotation = " (voided on resume)"
		}
		fmt.Fprintf(builder, "- %s%s: %s\n", record.Action.Kind, annotation, record.Outcome.Summary)
		if record.Outcome.Implementation != nil {
			for _, path := range record.Outcome.Implementation.FilesChanged {
				if path = strings.TrimSpace(path); path != "" {
					changedFiles[path] = struct{}{}
				}
			}
		}
		if record.Outcome.Review != nil {
			reviewOutcome = record.Outcome
			if record.Outcome.Review.Result == ReviewFindings {
				reviewFindings = append(reviewFindings, record.Outcome.Review.Findings...)
			}
		} else {
			observations = append(observations, record.Outcome.Observations...)
		}
	}
	for _, observation := range run.Observations {
		if strings.HasPrefix(observation, "observed_since_start:") || strings.HasPrefix(observation, "report_discrepancy:") {
			observations = appendUniqueString(observations, observation)
		}
	}

	if !run.FinalGitObserved {
		builder.WriteString("CLI Git observation: final worktree state was unavailable; no present-tense change claim is made.\n")
	} else if run.CurrentGit.ChangedFrom(run.InitialGit) {
		builder.WriteString(observedSinceStart + "\n")
		builder.WriteString(attributionUncertainty + "\n")
	} else {
		builder.WriteString("CLI Git observation: no difference from the run-start snapshot was observed.\n")
	}
	files := make([]string, 0, len(changedFiles))
	for path := range changedFiles {
		files = append(files, path)
	}
	sort.Strings(files)
	if len(files) > 0 {
		writeSummaryStrings(builder, "Files reported changed by Implement:", files)
	} else {
		builder.WriteString("No files were reported changed by Implement.\n")
	}
	if len(confirmedDecisions) > 0 {
		builder.WriteString("Confirmed decisions:\n")
		for _, answer := range confirmedDecisions {
			fmt.Fprintf(builder, "- action %s: %s\n", answer.ActionID, answer.Text)
		}
	}
	writeSummaryStrings(builder, "Observations:", observations)
	if len(reviewFindings) > 0 {
		builder.WriteString("Review findings:\n")
		for _, finding := range reviewFindings {
			fmt.Fprintf(builder, "- %s: %s — %s\n", finding.Location, finding.Summary, finding.Detail)
		}
	} else if reviewSkippedByUser {
		builder.WriteString("Review was skipped by the user.\n")
	} else if reviewOutcome != nil {
		fmt.Fprintf(builder, "Review report: %s: %s\n", reviewOutcome.Review.Result, reviewOutcome.Summary)
	} else if reviewVoided {
		builder.WriteString("A Review Action was dispatched but voided on resume before completion.\n")
	} else if !run.ReviewEnabled {
		builder.WriteString("Review was disabled for this run.\n")
	} else {
		builder.WriteString("Review was not run because no changed-code review Action was dispatched.\n")
	}
	if len(run.Activities) == 0 {
		builder.WriteString("No test, typecheck, build, or lint activity was reported.\n")
	} else {
		builder.WriteString("Reported technical activities:\n")
		for _, activity := range run.Activities {
			fmt.Fprintf(builder, "- %s: %s (exit %d): %s\n", activity.Kind, activity.Command, activity.ExitCode, activity.Summary)
		}
	}
	writeSummaryStrings(builder, "Skipped Actions:", skipped)
	writeSummaryStrings(builder, "Actions voided on resume:", voided)
	writeSummaryStrings(builder, "Known issues:", run.KnownIssues)
	writeSummaryStrings(builder, "Uncertainties:", run.Uncertainties)
	if run.InitialGit.PathCount > 0 {
		fmt.Fprintf(builder, "Pre-existing dirty path observations at Run start (count=%d; retained=%d; path_fingerprint=%s):\n", run.InitialGit.PathCount, len(run.InitialGit.PathObservations), run.InitialGit.PathFingerprint)
		for _, item := range run.InitialGit.PathObservations {
			fmt.Fprintf(builder, "- %s [%s %s; %s", item.Path, item.Category, item.State, item.Observation)
			if item.Size != nil {
				fmt.Fprintf(builder, "; size=%d", *item.Size)
			}
			if item.ContentSHA256 != "" {
				fmt.Fprintf(builder, "; content_sha256=%s", item.ContentSHA256)
			}
			builder.WriteString("]\n")
		}
		if run.InitialGit.DetailsTruncated {
			fmt.Fprintf(builder, "- %d additional path detail(s) omitted from the bounded projection.\n", run.InitialGit.PathCount-len(run.InitialGit.PathObservations))
		}
	}
	return builder.String()
}
