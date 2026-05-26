package stringutil

import (
	"slices"
	"strings"
)

// Unique removes duplicate strings while preserving insertion order.
// It does not trim whitespace, filter empty strings, or sort.
// Returns nil for empty or nil input.
func Unique(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(s))
	var result []string
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// UniqueSorted removes duplicate strings, trims whitespace from each entry,
// drops empty-after-trim entries, and returns the result sorted lexicographically.
// Returns nil for empty or nil input.
func UniqueSorted(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(s))
	var result []string
	for _, v := range s {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	if len(result) == 0 {
		return nil
	}
	slices.Sort(result)
	return result
}
