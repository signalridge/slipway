package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"slices"
	"strings"
)

// ComputeFileContentHash returns SHA-256 hex digest of a file's content.
func ComputeFileContentHash(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// ComputeInputHash returns canonical SHA-256 hash over normalized JSON payload.
// Used for task evidence input hashing in wave execution.
func ComputeInputHash(payload map[string]any) (string, error) {
	normalized := normalizeCanonical(payload)
	b, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func normalizeCanonical(v any) any {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		out := make(map[string]any, len(x))
		for _, k := range keys {
			out[k] = normalizeCanonical(x[k])
		}
		return out
	case []any:
		out := make([]any, 0, len(x))
		for _, item := range x {
			out = append(out, normalizeCanonical(item))
		}
		return out
	case string:
		return strings.ReplaceAll(x, "\r\n", "\n")
	default:
		return v
	}
}
