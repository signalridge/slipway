package progression

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSteadyStateFreshnessPathsDoNotConsumeFileModTime(t *testing.T) {
	t.Parallel()

	repoRoot := repositoryRootForFreshnessGuard(t)
	for _, rel := range freshnessGuardProductionFiles(t, repoRoot) {
		raw, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		source := string(raw)
		if !strings.Contains(source, "ModTime(") {
			continue
		}
		if rel == "internal/engine/progression/evidence_digests.go" &&
			modTimeCallsLimitedToFunctions(source,
				"func digestInputsChangedAfterVerdict(",
				"func digestInputChangedAfterVerdict(",
				"func digestInputPathChangedAfterVerdict(",
			) {
			continue
		}
		t.Fatalf("%s must not consume file mtimes on steady-state freshness paths", rel)
	}
}

func TestSteadyStateFreshnessPathsDoNotConsumeWallClockNow(t *testing.T) {
	t.Parallel()

	repoRoot := repositoryRootForFreshnessGuard(t)
	for _, rel := range freshnessGuardProductionFiles(t, repoRoot) {
		raw, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		source := string(raw)
		if !strings.Contains(source, "time.Now(") {
			continue
		}
		switch rel {
		case "internal/engine/progression/wave_sync.go":
			if sourceTokensLimitedToFunctions(source, []string{"time.Now("}, "func BuildExecutionSummary(") {
				continue
			}
		case "internal/state/wave_execution.go":
			if sourceTokensLimitedToFunctions(source, []string{"time.Now("}, "func MaterializeWavePlan(") {
				continue
			}
		}
		t.Fatalf("%s must not consume wall-clock time on steady-state freshness paths", rel)
	}
}

func TestGenericEvidenceFreshnessDoesNotUseTimestampOrdering(t *testing.T) {
	t.Parallel()

	repoRoot := repositoryRootForFreshnessGuard(t)
	raw, err := os.ReadFile(filepath.Join(repoRoot, "internal/engine/context/context.go"))
	if err != nil {
		t.Fatalf("read context.go: %v", err)
	}
	source := string(raw)
	if strings.Contains(source, "EvidenceTimestamp.Before") ||
		strings.Contains(source, "LatestRelevantUpdateAt.After") ||
		strings.Contains(source, "LatestRelevantUpdateAt.Before") {
		t.Fatalf("generic evidence freshness must not compare timestamp ordering")
	}
	raw, err = os.ReadFile(filepath.Join(repoRoot, "internal/state/execution_summary.go"))
	if err != nil {
		t.Fatalf("read execution_summary.go: %v", err)
	}
	if strings.Contains(string(raw), "func latestExecutionRelevantUpdateAt(") {
		t.Fatalf("execution-summary freshness must not keep artifact-clock baseline helpers")
	}
}

func TestAuthorityTimestampOrderingIsLimitedToCloseoutProofOrdering(t *testing.T) {
	t.Parallel()

	repoRoot := repositoryRootForFreshnessGuard(t)
	raw, err := os.ReadFile(filepath.Join(repoRoot, "internal/engine/progression/authority.go"))
	if err != nil {
		t.Fatalf("read authority.go: %v", err)
	}
	if !sourceTokensLimitedToFunctions(string(raw),
		[]string{".Before(", ".After("},
		"func closeoutGoalVerificationReuseBlockers(",
		"func closeoutChainOrderBlockers(",
	) {
		t.Fatalf("authority timestamp ordering must stay limited to closeout proof-ordering gates")
	}
}

func repositoryRootForFreshnessGuard(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func freshnessGuardProductionFiles(t *testing.T, repoRoot string) []string {
	t.Helper()

	files := []string{
		"internal/engine/context/context.go",
		"internal/state/execution_repair.go",
		"internal/state/execution_summary.go",
		"internal/state/wave_execution.go",
	}
	progressDir := filepath.Join(repoRoot, "internal", "engine", "progression")
	err := filepath.WalkDir(progressDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk progression files: %v", err)
	}
	return files
}

func modTimeCallsLimitedToFunctions(source string, signatures ...string) bool {
	return sourceTokensLimitedToFunctions(source, []string{"ModTime("}, signatures...)
}

func sourceTokensLimitedToFunctions(source string, tokens []string, signatures ...string) bool {
	lines := strings.Split(source, "\n")
	allowed := make([]bool, len(lines))
	for _, signature := range signatures {
		start := -1
		for idx, line := range lines {
			if strings.Contains(line, signature) {
				start = idx
				break
			}
		}
		if start < 0 {
			return false
		}
		end := len(lines)
		for idx := start + 1; idx < len(lines); idx++ {
			if strings.HasPrefix(lines[idx], "func ") {
				end = idx
				break
			}
		}
		for idx := start; idx < end; idx++ {
			allowed[idx] = true
		}
	}
	for idx, line := range lines {
		for _, token := range tokens {
			if strings.Contains(line, token) && !allowed[idx] {
				return false
			}
		}
	}
	return true
}
