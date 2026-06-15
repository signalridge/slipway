package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func makeMergeSARIFCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "merge-sarif RAW_DIR OUTPUT_FILE",
		Short: "Merge SARIF files deterministically",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMergeSARIF(cmd, args[0], args[1])
		},
	}
}

type sarifRunGroup struct {
	key  sarifRunGroupKey
	runs []map[string]any
}

type sarifRunGroupKey struct {
	tool    string
	profile string
	workdir string
}

func runMergeSARIF(cmd *cobra.Command, rawDir, outputFile string) error {
	info, err := os.Stat(rawDir)
	if err != nil || !info.IsDir() {
		return newPreconditionError("merge_sarif_raw_dir_invalid", fmt.Sprintf("%s is not a directory", rawDir), "Pass a directory containing *.sarif files.", "", nil)
	}

	entries, err := os.ReadDir(rawDir)
	if err != nil {
		return err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sarif" {
			continue
		}
		files = append(files, filepath.Join(rawDir, entry.Name()))
	}
	sort.Strings(files)
	if len(files) == 0 {
		return newPreconditionError("merge_sarif_no_files", fmt.Sprintf("no SARIF files found in %s", rawDir), "Place at least one *.sarif file in RAW_DIR.", "", nil)
	}

	var docs []map[string]any
	var skipped []string
	for _, file := range files {
		raw, err := os.ReadFile(file) // #nosec G304 -- explicit user-supplied helper input path.
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", file, err))
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", file, err))
			continue
		}
		if version := stringField(doc, "version"); version != "2.1.0" {
			return newPreconditionError(
				"merge_sarif_version_unsupported",
				fmt.Sprintf("%s has unsupported SARIF version %q", file, version),
				"Pass SARIF 2.1.0 input files.",
				"",
				map[string]any{"path": file, "version": version},
			)
		}
		docs = append(docs, doc)
	}
	if len(skipped) > 0 {
		return newPreconditionError(
			"merge_sarif_parse_failed",
			fmt.Sprintf("%d of %d SARIF file(s) failed to parse; refusing to emit an incomplete merge", len(skipped), len(files)),
			"Fix or remove unparseable SARIF inputs and rerun; partial merges must not be treated as complete scan evidence.",
			"",
			map[string]any{"skipped": skipped},
		)
	}

	out := map[string]any{
		"version": "2.1.0",
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"runs":    []any{},
	}
	out["runs"] = mergedSARIFRuns(docs)

	if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
		return err
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(outputFile, raw, fs.FileMode(0o644)); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Merged %d SARIF file(s) -> %s (%d result(s))\n", len(docs), outputFile, countSARIFResults(out))
	return nil
}

func mergedSARIFRuns(docs []map[string]any) []any {
	groupsByKey := map[sarifRunGroupKey]int{}
	var groups []sarifRunGroup
	for _, doc := range docs {
		for _, run := range mapSlice(doc, "runs") {
			key := sarifGroupKey(run)
			idx, ok := groupsByKey[key]
			if !ok {
				idx = len(groups)
				groupsByKey[key] = idx
				groups = append(groups, sarifRunGroup{key: key})
			}
			groups[idx].runs = append(groups[idx].runs, run)
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		a, b := groups[i].key, groups[j].key
		return strings.Compare(a.tool+"\x00"+a.profile+"\x00"+a.workdir, b.tool+"\x00"+b.profile+"\x00"+b.workdir) < 0
	})

	out := make([]any, 0, len(groups))
	for _, group := range groups {
		first := map[string]any{}
		if len(group.runs) > 0 {
			first = group.runs[0]
		}

		// Build the merged, deduped, id-sorted driver.rules table for the group
		// and an id->mergedIndex map so per-result ruleIndex/rule.index can be
		// remapped in lockstep (sarif-merge.md invariant 2).
		mergedRules := mergedSARIFRules(group.runs)
		ruleIndexByID := make(map[string]int, len(mergedRules))
		for i, rule := range mergedRules {
			if ruleMap, ok := rule.(map[string]any); ok {
				if id := stringField(ruleMap, "id"); id != "" {
					ruleIndexByID[id] = i
				}
			}
		}

		// Approach for artifacts (sarif-merge.md invariant 3): build a MERGED
		// artifacts table for the group, deduped by artifactLocation.uri in
		// deterministic first-seen order, and remap every artifactLocation.index
		// in every result into that merged table. The merged run's `artifacts`
		// is set to this table so surviving indices resolve correctly.
		mergedArtifacts, artifactIndexByURI := mergedSARIFArtifacts(group.runs)

		merged := map[string]any{
			"tool":    cloneJSONMap(mapField(first, "tool")),
			"results": mergedSARIFResults(group.runs, ruleIndexByID, artifactIndexByURI),
		}
		if len(merged["tool"].(map[string]any)) == 0 {
			merged["tool"] = map[string]any{"driver": map[string]any{"name": "merge-sarif", "rules": []any{}}}
		}
		driver := mapField(merged["tool"].(map[string]any), "driver")
		if len(driver) == 0 {
			driver = map[string]any{"name": "merge-sarif"}
		}
		driver["rules"] = mergedRules
		merged["tool"].(map[string]any)["driver"] = driver
		copyOptionalSARIFFields(merged, first)
		// Override any first-run artifacts table carried by copyOptionalSARIFFields
		// with the merged table that the remapped indices point into.
		if len(mergedArtifacts) > 0 {
			merged["artifacts"] = mergedArtifacts
		} else {
			delete(merged, "artifacts")
		}
		out = append(out, merged)
	}
	return out
}

func sarifGroupKey(run map[string]any) sarifRunGroupKey {
	invocations := mapSlice(run, "invocations")
	var invocation map[string]any
	if len(invocations) > 0 {
		invocation = invocations[0]
	}
	props := mapField(invocation, "properties")
	profile := stringField(props, "scan_profile")
	if profile == "" {
		profile = stringField(props, "profile")
	}
	return sarifRunGroupKey{
		tool:    stringField(mapField(mapField(run, "tool"), "driver"), "name"),
		profile: profile,
		workdir: stringField(mapField(invocation, "workingDirectory"), "uri"),
	}
}

func mergedSARIFRules(runs []map[string]any) []any {
	seen := map[string]bool{}
	var rules []map[string]any
	for _, run := range runs {
		for _, rule := range mapSlice(mapField(mapField(run, "tool"), "driver"), "rules") {
			id := stringField(rule, "id")
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			rules = append(rules, cloneJSONMap(rule))
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		return stringField(rules[i], "id") < stringField(rules[j], "id")
	})
	out := make([]any, 0, len(rules))
	for _, rule := range rules {
		out = append(out, rule)
	}
	return out
}

// mergedSARIFArtifacts builds a merged artifacts table for the group, deduped
// by artifactLocation.uri in deterministic first-seen order across the group's
// runs. Existing run.artifacts entries keep their metadata; result locations
// with a URI but no source artifact entry get a minimal artifact record so
// scanners that omit artifacts[] still leave downstream readers a stable table.
// It returns the table plus a uri->mergedIndex map used to remap result
// artifactLocation.index values.
func mergedSARIFArtifacts(runs []map[string]any) ([]any, map[string]int) {
	indexByURI := map[string]int{}
	var artifacts []any
	addArtifact := func(uri string, artifact map[string]any) {
		if uri == "" || hasKey(indexByURI, uri) {
			return
		}
		indexByURI[uri] = len(artifacts)
		if len(artifact) > 0 {
			artifacts = append(artifacts, cloneJSONMap(artifact))
			return
		}
		artifacts = append(artifacts, map[string]any{
			"location": map[string]any{"uri": uri},
		})
	}
	for _, run := range runs {
		for _, artifact := range mapSlice(run, "artifacts") {
			uri := stringField(mapField(artifact, "location"), "uri")
			addArtifact(uri, artifact)
		}
		for _, result := range mapSlice(run, "results") {
			walkArtifactLocations(result, func(loc map[string]any) {
				addArtifact(stringField(loc, "uri"), nil)
			})
		}
	}
	return artifacts, indexByURI
}

// mergedSARIFResults clones each result from its SOURCE run, remapping rule and
// artifact indices into the merged tables before dedupe/sort. Each result's
// ruleIndex/rule.index is rewritten via the source run's rules table into the
// merged id-sorted table, and each artifactLocation.index is rewritten via the
// source run's artifacts table into the merged artifacts table. ruleId is
// backfilled from the source run when only ruleIndex was present so consumers
// are never forced onto the index, and stale indices are removed rather than
// left pointing at the wrong entry.
func mergedSARIFResults(runs []map[string]any, ruleIndexByID map[string]int, artifactIndexByURI map[string]int) []any {
	seen := map[string]bool{}
	var results []map[string]any
	for _, run := range runs {
		sourceRules := mapSlice(mapField(mapField(run, "tool"), "driver"), "rules")
		sourceArtifacts := mapSlice(run, "artifacts")
		for _, result := range mapSlice(run, "results") {
			cloned := cloneJSONMap(result)
			remapResultRuleIndex(cloned, sourceRules, ruleIndexByID)
			remapResultArtifactIndices(cloned, sourceArtifacts, artifactIndexByURI)
			key := sarifResultKey(cloned)
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, cloned)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return sarifResultLess(results[i], results[j])
	})
	out := make([]any, 0, len(results))
	for _, result := range results {
		out = append(out, result)
	}
	return out
}

// remapResultRuleIndex backfills ruleId from the source run's rules[ruleIndex]
// when absent, then rewrites ruleIndex and (if present) rule.index to the
// merged table position. If the rule id cannot be resolved in the merged table,
// the stale ruleIndex/rule.index is removed (never left pointing at the wrong
// rule); ruleId is retained.
func remapResultRuleIndex(result map[string]any, sourceRules []map[string]any, ruleIndexByID map[string]int) {
	ruleID := stringField(result, "ruleId")
	_, hasTopIndex := result["ruleIndex"]
	ruleObj, _ := result["rule"].(map[string]any)
	hasRuleObjIndex := false
	if ruleObj != nil {
		_, hasRuleObjIndex = ruleObj["index"]
	}

	// Backfill ruleId from the source run's rules table when only an index is
	// present, using whichever index the result carries.
	if ruleID == "" {
		if hasTopIndex {
			ruleID = sourceRuleIDAt(sourceRules, intField(result, "ruleIndex"))
		} else if hasRuleObjIndex {
			ruleID = sourceRuleIDAt(sourceRules, intField(ruleObj, "index"))
		}
		if ruleID != "" {
			result["ruleId"] = ruleID
		}
	}

	mergedIdx, resolvable := ruleIndexByID[ruleID]
	if ruleID != "" && resolvable {
		if hasTopIndex {
			result["ruleIndex"] = mergedIdx
		}
		if hasRuleObjIndex {
			ruleObj["index"] = mergedIdx
			result["rule"] = ruleObj
		}
		return
	}
	// Unresolvable: drop stale indices so none survives pointing at a wrong rule.
	delete(result, "ruleIndex")
	if hasRuleObjIndex {
		delete(ruleObj, "index")
		result["rule"] = ruleObj
	}
}

func sourceRuleIDAt(sourceRules []map[string]any, idx int) string {
	if idx < 0 || idx >= len(sourceRules) {
		return ""
	}
	return stringField(sourceRules[idx], "id")
}

// remapResultArtifactIndices rewrites every artifactLocation.index found under a
// result (in locations[], relatedLocations[], and codeFlows thread locations)
// from the source run's artifacts table into the merged artifacts table via the
// artifactLocation.uri. A location that already has a URI but no index gets the
// merged index added. When the URI cannot be resolved in the merged table the
// stale index is removed (URI retained) so no index survives pointing at a wrong
// entry.
func remapResultArtifactIndices(result map[string]any, sourceArtifacts []map[string]any, artifactIndexByURI map[string]int) {
	walkArtifactLocations(result, func(loc map[string]any) {
		idxRaw, hasIdx := loc["index"]
		uri := stringField(loc, "uri")
		if uri == "" && hasIdx {
			// Recover uri from the source artifacts table so the remap can proceed.
			uri = sourceArtifactURIAt(sourceArtifacts, toInt(idxRaw))
			if uri != "" {
				loc["uri"] = uri
			}
		}
		if mergedIdx, ok := artifactIndexByURI[uri]; ok && uri != "" {
			loc["index"] = mergedIdx
			return
		}
		if hasIdx {
			delete(loc, "index")
		}
	})
}

func sourceArtifactURIAt(sourceArtifacts []map[string]any, idx int) string {
	if idx < 0 || idx >= len(sourceArtifacts) {
		return ""
	}
	return stringField(mapField(sourceArtifacts[idx], "location"), "uri")
}

// walkArtifactLocations invokes fn on every artifactLocation object reachable
// from a result. It recurses generically through maps and slices so any nested
// physicalLocation.artifactLocation (locations, relatedLocations, codeFlows,
// stacks, etc.) is covered without enumerating each SARIF container.
func walkArtifactLocations(node any, fn func(map[string]any)) {
	switch typed := node.(type) {
	case map[string]any:
		if loc, ok := typed["artifactLocation"].(map[string]any); ok {
			fn(loc)
		}
		for _, child := range typed {
			walkArtifactLocations(child, fn)
		}
	case []any:
		for _, child := range typed {
			walkArtifactLocations(child, fn)
		}
	}
}

func hasKey(m map[string]int, key string) bool {
	_, ok := m[key]
	return ok
}

func toInt(raw any) int {
	switch v := raw.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

// sarifResultFields extracts the (ruleId, primary-location uri, startLine)
// tuple used both for dedupe keying and ordering. Centralizing the extraction
// keeps the dedupe key and the sort comparator reading the same fields.
func sarifResultFields(result map[string]any) (ruleID, uri string, line int) {
	location := map[string]any{}
	locations := mapSlice(result, "locations")
	if len(locations) > 0 {
		location = locations[0]
	}
	physical := mapField(location, "physicalLocation")
	uri = stringField(mapField(physical, "artifactLocation"), "uri")
	line = intField(mapField(physical, "region"), "startLine")
	ruleID = stringField(result, "ruleId")
	return ruleID, uri, line
}

func sarifResultKey(result map[string]any) string {
	ruleID, uri, line := sarifResultFields(result)
	return ruleID + "\x00" + uri + "\x00" + strconv.Itoa(line)
}

// sarifResultLess orders merged results by ruleId, then artifact uri, then
// numeric startLine. Comparing startLine as an int (rather than the lexical
// dedupe key) keeps line 2 ahead of line 10.
func sarifResultLess(a, b map[string]any) bool {
	ruleA, uriA, lineA := sarifResultFields(a)
	ruleB, uriB, lineB := sarifResultFields(b)
	if ruleA != ruleB {
		return ruleA < ruleB
	}
	if uriA != uriB {
		return uriA < uriB
	}
	return lineA < lineB
}

func copyOptionalSARIFFields(dst, src map[string]any) {
	for _, key := range []string{"invocations", "artifacts", "originalUriBaseIds", "columnKind"} {
		if value, ok := src[key]; ok {
			dst[key] = value
		}
	}
}

func countSARIFResults(doc map[string]any) int {
	count := 0
	for _, run := range mapSlice(doc, "runs") {
		count += len(mapSlice(run, "results"))
	}
	return count
}

func mapSlice(m map[string]any, key string) []map[string]any {
	raw, _ := m[key].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if mapped, ok := item.(map[string]any); ok {
			out = append(out, mapped)
		}
	}
	return out
}

func mapField(m map[string]any, key string) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	if out, ok := m[key].(map[string]any); ok {
		return out
	}
	return map[string]any{}
}

func stringField(m map[string]any, key string) string {
	if raw, ok := m[key].(string); ok {
		return raw
	}
	return ""
}

func intField(m map[string]any, key string) int {
	switch raw := m[key].(type) {
	case int:
		return raw
	case float64:
		return int(raw)
	case json.Number:
		n, _ := raw.Int64()
		return int(n)
	default:
		return 0
	}
}

func cloneJSONMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}
