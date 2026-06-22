package wave

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/model"
)

type Node struct {
	TaskID      string         `json:"task_id"`
	Objective   string         `json:"objective,omitempty"`
	DependsOn   []string       `json:"depends_on,omitempty"`
	TargetFiles []string       `json:"target_files,omitempty"`
	TaskKind    model.TaskKind `json:"task_kind,omitempty"`
}

type Wave struct {
	Nodes []Node `json:"nodes"`
}

// PlanWaves computes the wave assignment for the given tasks from their
// declared depends_on edges and their target_files. Nothing is declared by
// the author: wave(task) = 1 for roots, otherwise max(wave of each
// dependency) + 1 before conflict adjustment. Waves are then filled in task-ID
// order from tasks whose dependencies are already in earlier waves, deferring
// any task that conflicts with an already accepted task in the current wave.
// Conflicts are a same-wave-only concern because waves run sequentially. Depth
// is minimal for pure dependency constraints; conflict bumping is deterministic
// greedy placement, not a depth-optimal schedule (that is graph-coloring-hard).
// The plan is deterministic across input orderings.
func PlanWaves(nodes []Node) ([]Wave, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	nodeByID := map[string]Node{}
	for _, node := range nodes {
		if err := model.ValidateTaskID(node.TaskID); err != nil {
			return nil, err
		}
		if _, exists := nodeByID[node.TaskID]; exists {
			return nil, fmt.Errorf("duplicate task_id %q", node.TaskID)
		}
		nodeByID[node.TaskID] = node
	}

	for _, node := range nodes {
		for _, dep := range node.DependsOn {
			if _, exists := nodeByID[dep]; !exists {
				return nil, fmt.Errorf("task %q depends on unknown task %q", node.TaskID, dep)
			}
		}
	}

	ordered, err := topologicalTaskOrder(nodeByID)
	if err != nil {
		return nil, err
	}

	taskIDs := append([]string(nil), ordered...)
	slices.Sort(taskIDs)
	assignedWave := map[string]int{}
	remaining := map[string]struct{}{}
	for _, taskID := range taskIDs {
		remaining[taskID] = struct{}{}
	}

	waves := []Wave{}
	for waveIndex := 1; len(remaining) > 0; waveIndex++ {
		layer := []Node{}
		for _, taskID := range taskIDs {
			if _, pending := remaining[taskID]; !pending {
				continue
			}
			node := nodeByID[taskID]
			if !dependenciesAssignedBeforeWave(node, assignedWave, waveIndex) {
				continue
			}
			if nodeConflictsWithWave(layer, node) {
				continue
			}
			layer = append(layer, node)
			assignedWave[taskID] = waveIndex
			delete(remaining, taskID)
		}

		if len(layer) == 0 {
			return nil, fmt.Errorf("no schedulable tasks for wave %d", waveIndex)
		}
		// Internal invariant: conflict-driven placement above must have
		// produced conflict-free waves; fail closed if it ever does not.
		if err := validateWaveStaticConflicts(waveIndex, layer); err != nil {
			return nil, err
		}
		waves = append(waves, Wave{Nodes: layer})
	}
	return waves, nil
}

func dependenciesAssignedBeforeWave(node Node, assignedWave map[string]int, waveIndex int) bool {
	for _, dep := range node.DependsOn {
		depWave, ok := assignedWave[dep]
		if !ok || depWave >= waveIndex {
			return false
		}
	}
	return true
}

// topologicalTaskOrder returns every task ID in dependency order with
// task-ID-ordered tiebreaks (Kahn's algorithm popping the smallest ready
// ID), so wave assignment is deterministic regardless of input order.
// Duplicate depends_on entries are tolerated; an unresolvable remainder is a
// dependency cycle and fails the plan.
func topologicalTaskOrder(nodeByID map[string]Node) ([]string, error) {
	taskIDs := make([]string, 0, len(nodeByID))
	for taskID := range nodeByID {
		taskIDs = append(taskIDs, taskID)
	}
	slices.Sort(taskIDs)

	pendingDeps := map[string]int{}
	dependents := map[string][]string{}
	for _, taskID := range taskIDs {
		seenDeps := map[string]struct{}{}
		for _, dep := range nodeByID[taskID].DependsOn {
			if _, dup := seenDeps[dep]; dup {
				continue
			}
			seenDeps[dep] = struct{}{}
			pendingDeps[taskID]++
			dependents[dep] = append(dependents[dep], taskID)
		}
	}

	ready := make([]string, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		if pendingDeps[taskID] == 0 {
			ready = append(ready, taskID)
		}
	}

	ordered := make([]string, 0, len(taskIDs))
	for len(ready) > 0 {
		taskID := ready[0]
		ready = ready[1:]
		ordered = append(ordered, taskID)
		for _, dependent := range dependents[taskID] {
			pendingDeps[dependent]--
			if pendingDeps[dependent] == 0 {
				insertAt, _ := slices.BinarySearch(ready, dependent)
				ready = slices.Insert(ready, insertAt, dependent)
			}
		}
	}

	if len(ordered) != len(taskIDs) {
		stuck := make([]string, 0, len(taskIDs)-len(ordered))
		for _, taskID := range taskIDs {
			if pendingDeps[taskID] > 0 {
				stuck = append(stuck, taskID)
			}
		}
		return nil, fmt.Errorf("depends_on cycle detected among tasks %s; remove a circular depends_on reference so every task can be ordered", strings.Join(stuck, ", "))
	}
	return ordered, nil
}

// nodeConflictsWithWave reports whether the candidate's target files overlap
// any target file already owned by another task assigned to the wave.
func nodeConflictsWithWave(occupants []Node, candidate Node) bool {
	for _, candidateFile := range candidate.TargetFiles {
		candidateTarget := normalizeTargetFileForConflict(candidateFile)
		for _, occupant := range occupants {
			for _, occupantFile := range occupant.TargetFiles {
				if targetFilesConflict(normalizeTargetFileForConflict(occupantFile), candidateTarget) {
					return true
				}
			}
		}
	}
	return false
}

// TargetCoversPath reports whether file is covered by any entry in targets,
// using the same directional scope semantics the wave planner's conflict
// detection relies on. A target covers file when, after both are normalized as
// public paths (case-folded), the target equals the file, is a parent
// directory that contains it, or is a glob pattern that matches it. Coverage is
// the primitive the changed-file scope-escape audit builds on; conflict
// detection is mutual coverage. Empty targets or an empty file never match.
func TargetCoversPath(targets []string, file string) bool {
	fn := normalizeTargetFileForConflict(file)
	if fn == "" {
		return false
	}
	for _, target := range targets {
		tn := normalizeTargetFileForConflict(target)
		if tn == "" {
			continue
		}
		if tn == fn || targetFileContains(tn, fn) {
			return true
		}
		if targetHasPatternMeta(tn) && targetPatternCovers(tn, fn) {
			return true
		}
	}
	return false
}

// CanonicalConflictPath returns the canonical key a path collapses to under the
// wave planner's conflict semantics: normalized as a public path and case-folded
// so two spellings of the same file (slash/backslash, mixed case) compare equal.
// It is the single source of "same file" for both coverage (TargetCoversPath)
// and the same-wave changed-file overlap audit, so overlap detection buckets by
// exactly the identity the planner uses instead of trusting raw recorded
// strings. An empty or "." path returns "".
func CanonicalConflictPath(file string) string {
	return normalizeTargetFileForConflict(file)
}

// AnalyzeWaveNarrowingCauses returns deterministic, non-blocking advisory
// strings that explain why a tasks plan parallelizes poorly. It is a view-layer
// signal only: it never blocks execution and is excluded from wave-plan.yaml
// and every freshness hash. The advisories are conservative, high-confidence
// cues plan-audit can cite when rejecting narrative dependencies or over-broad
// targets:
//   - broad_target_files:<task_id> for any node whose target_files contain a
//     glob/pattern meta or a directory target (no file extension), reusing the
//     same pattern-meta detection the conflict predicates rely on so the cue and
//     the planner agree on what "broad" means.
//   - fully_serial_plan when the dependency-only critical path spans every node
//     (max dependency depth == len(nodes)). This intentionally excludes
//     serialization that the planner introduces purely to resolve file
//     conflicts: a plan that runs one-task-per-wave only because the tasks share
//     concrete file targets is honest scheduling, not a narrowing cause.
//
// Serial detection deliberately uses dependency depth rather than the
// materialized wave count so conflict-driven serialization is not reported.
func AnalyzeWaveNarrowingCauses(nodes []Node) []string {
	if len(nodes) == 0 {
		return nil
	}

	advisories := []string{}

	broadTaskIDs := []string{}
	for _, node := range nodes {
		if nodeHasBroadTarget(node) {
			broadTaskIDs = append(broadTaskIDs, node.TaskID)
		}
	}
	slices.Sort(broadTaskIDs)
	for _, taskID := range broadTaskIDs {
		advisories = append(advisories, "broad_target_files:"+taskID)
	}

	// A single-task plan has nothing to parallelize, so a critical path equal to
	// the (one) node count is honest scheduling, not a narrowing cause: require
	// more than one node before reporting a fully serial dependency chain.
	if len(nodes) > 1 && dependencyCriticalPathLength(nodes) == len(nodes) {
		advisories = append(advisories, "fully_serial_plan")
	}

	if len(advisories) == 0 {
		return nil
	}
	return advisories
}

// nodeHasBroadTarget reports whether any of the node's target_files is a glob
// pattern or directory-like scope. Both are over-broad scopes that weaken the
// same-wave file-disjointness guarantee. Because target files are normalized for
// persisted plans, this stays deliberately heuristic and non-blocking: explicit
// directory markers, common repository directory names, *.d directory
// conventions, and extensionless non-conventional targets all produce advisories.
func nodeHasBroadTarget(node Node) bool {
	for _, file := range node.TargetFiles {
		if explicitDirectoryTarget(file) {
			return true
		}
		target := normalizeTargetFileForConflict(file)
		if target == "" {
			continue
		}
		if targetHasPatternMeta(target) {
			return true
		}
		if targetLooksDirectoryLike(target) {
			return true
		}
	}
	return false
}

func explicitDirectoryTarget(raw string) bool {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	return strings.Trim(value, "/") != "" && strings.HasSuffix(value, "/")
}

func targetLooksDirectoryLike(target string) bool {
	base := path.Base(target)
	if isConventionalExtensionlessFile(target) {
		return false
	}
	if _, ok := conventionalDirectoryTargets[base]; ok {
		return true
	}
	if strings.HasSuffix(base, ".d") {
		return true
	}
	if path.Ext(target) != "" {
		return false
	}
	// Nested extensionless paths are ambiguous: they can be directories, but they
	// are also common executable/script file names such as scripts/deploy. Keep the
	// advisory high-confidence and only infer bare top-level extensionless targets
	// as directory-like unless the caller supplied an explicit trailing slash.
	return !strings.Contains(target, "/")
}

// conventionalExtensionlessFiles are well-known repository files that carry no
// extension yet name a single concrete file, so they are excluded from the
// directory-target heuristic in nodeHasBroadTarget. Matched on the lower-cased
// path basename, so the set is case-insensitive and directory-independent
// (e.g. .github/CODEOWNERS or build/Dockerfile both match).
var conventionalExtensionlessFiles = map[string]struct{}{
	"makefile":      {},
	"dockerfile":    {},
	"containerfile": {},
	"license":       {},
	"licence":       {},
	"copying":       {},
	"notice":        {},
	"readme":        {},
	"changelog":     {},
	"authors":       {},
	"contributors":  {},
	"codeowners":    {},
	"gemfile":       {},
	"rakefile":      {},
	"procfile":      {},
	"brewfile":      {},
	"vagrantfile":   {},
	"caddyfile":     {},
	"justfile":      {},
	"taskfile":      {},
	"jenkinsfile":   {},
	"earthfile":     {},
}

// conventionalDirectoryTargets are common extension-looking repository
// directory names that path.Ext would otherwise classify as file-like after
// target normalization strips explicit trailing slashes.
var conventionalDirectoryTargets = map[string]struct{}{
	".github":       {},
	".gitlab":       {},
	".gitea":        {},
	".vscode":       {},
	".idea":         {},
	".devcontainer": {},
	".config":       {},
}

// isConventionalExtensionlessFile reports whether target's basename is a
// well-known extensionless single-file name. target is already lower-cased and
// normalized by normalizeTargetFileForConflict.
func isConventionalExtensionlessFile(target string) bool {
	_, ok := conventionalExtensionlessFiles[path.Base(target)]
	return ok
}

// dependencyCriticalPathLength returns the longest chain of depends_on edges,
// counted in nodes: a root (no depends_on) has depth 1, and any other node has
// depth max(depth(dependency))+1. Unknown dependency references contribute no
// depth. Cyclic plans never reach this analyzer (PlanWaves rejects them first),
// so a simple memoized walk over the declared edges is sufficient.
func dependencyCriticalPathLength(nodes []Node) int {
	nodeByID := make(map[string]Node, len(nodes))
	for _, node := range nodes {
		nodeByID[node.TaskID] = node
	}

	depthByID := make(map[string]int, len(nodes))
	var depth func(taskID string) int
	depth = func(taskID string) int {
		if cached, ok := depthByID[taskID]; ok {
			return cached
		}
		// Guard against accidental self/cyclic references defensively even
		// though PlanWaves rejects cycles upstream.
		depthByID[taskID] = 1
		node, known := nodeByID[taskID]
		if !known {
			return 1
		}
		best := 0
		for _, dep := range node.DependsOn {
			if dep == taskID {
				continue
			}
			if _, known := nodeByID[dep]; !known {
				// Unknown dependency references contribute no depth. PlanWaves
				// rejects unknown depends_on upstream, so this only guards direct
				// analyzer calls from inflating the critical path.
				continue
			}
			if depDepth := depth(dep); depDepth > best {
				best = depDepth
			}
		}
		result := best + 1
		depthByID[taskID] = result
		return result
	}

	longest := 0
	for _, node := range nodes {
		if d := depth(node.TaskID); d > longest {
			longest = d
		}
	}
	return longest
}

func validateWaveStaticConflicts(waveIndex int, nodes []Node) error {
	type targetOwner struct {
		target string
		taskID string
	}

	targetOwners := []targetOwner{}
	for _, node := range nodes {
		for _, file := range node.TargetFiles {
			target := normalizeTargetFileForConflict(file)
			for _, existing := range targetOwners {
				if existing.taskID == node.TaskID {
					continue
				}
				if targetFilesConflict(existing.target, target) {
					return fmt.Errorf("wave %d has static target conflict: %q targets %q and %q targets %q", waveIndex, existing.taskID, existing.target, node.TaskID, target)
				}
			}
			targetOwners = append(targetOwners, targetOwner{target: target, taskID: node.TaskID})
		}
	}
	return nil
}

func targetFilesConflict(left, right string) bool {
	if left == right || targetFileContains(left, right) || targetFileContains(right, left) {
		return true
	}
	return targetPatternConflicts(left, right)
}

func targetFileContains(parent, child string) bool {
	if parent == "" || child == "" {
		return false
	}
	if parent == "." || parent == "/" {
		return child != parent
	}
	return strings.HasPrefix(child, parent+"/")
}

func normalizeTargetFileForConflict(file string) string {
	normalized := model.NormalizePublicPath(file)
	if normalized == "" {
		return ""
	}
	// Be conservative across case-insensitive developer filesystems: same-wave
	// targets that differ only by case must not be auto-parallelized.
	return strings.ToLower(normalized)
}

func targetPatternConflicts(left, right string) bool {
	leftPattern := targetHasPatternMeta(left)
	rightPattern := targetHasPatternMeta(right)
	switch {
	case leftPattern && rightPattern:
		return targetPatternPrefixesOverlap(left, right)
	case leftPattern:
		return targetPatternMatches(left, right)
	case rightPattern:
		return targetPatternMatches(right, left)
	default:
		return false
	}
}

func targetHasPatternMeta(target string) bool {
	return strings.ContainsAny(target, "*?[")
}

func targetPatternMatches(pattern, target string) bool {
	// Malformed glob targets are treated as broad conflicts so a bad target_files
	// pattern cannot accidentally pack unsafe same-wave parallel work.
	return targetPatternMatchesWithMalformedPolicy(pattern, target, true)
}

func targetPatternCovers(pattern, target string) bool {
	// The same malformed glob must not grant changed-file coverage authority:
	// fail closed by reporting the changed file as out of scope.
	return targetPatternMatchesWithMalformedPolicy(pattern, target, false)
}

func targetPatternMatchesWithMalformedPolicy(pattern, target string, malformedPatternMatches bool) bool {
	if pattern == "" || target == "" {
		return false
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return prefix == "" || target == prefix || targetFileContains(prefix, target)
	}
	if strings.Contains(pattern, "**") {
		return targetOverlapsPatternStaticPrefix(pattern, target)
	}
	matched, err := path.Match(pattern, target)
	if err != nil {
		return malformedPatternMatches
	}
	return matched
}

func targetPatternPrefixesOverlap(left, right string) bool {
	leftPrefix := targetPatternStaticPrefix(left)
	rightPrefix := targetPatternStaticPrefix(right)
	if leftPrefix == "" || rightPrefix == "" {
		return true
	}
	return leftPrefix == rightPrefix ||
		targetFileContains(leftPrefix, rightPrefix) ||
		targetFileContains(rightPrefix, leftPrefix)
}

func targetOverlapsPatternStaticPrefix(pattern, target string) bool {
	prefix := targetPatternStaticPrefix(pattern)
	return prefix == "" || target == prefix || targetFileContains(prefix, target)
}

func targetPatternStaticPrefix(pattern string) string {
	patternIndex := strings.IndexAny(pattern, "*?[")
	if patternIndex < 0 {
		return strings.TrimSuffix(pattern, "/")
	}
	prefix := pattern[:patternIndex]
	slashIndex := strings.LastIndex(prefix, "/")
	if slashIndex < 0 {
		return ""
	}
	return strings.TrimSuffix(prefix[:slashIndex+1], "/")
}
