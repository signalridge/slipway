// Package jsonstrict owns the structural JSON scanning shared by the
// autopilot, runstore, and adapter packages: a single recursive walk that
// rejects duplicate object keys, structurally invalid JSON, and any trailing
// value or data after the first JSON document. It is deliberately a structural
// scanner only. Each caller composes its own policy on top (UTF-8/BOM/empty
// rejection, schema reflection, DisallowUnknownFields, decoding into a typed
// target), because those rules differ per surface and must not be unified into
// a single "strict decode" that would silently change contracts.
package jsonstrict

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ScanStructure scans one JSON value and rejects duplicate object keys,
// malformed structure, and any trailing value or data after it. It uses
// json.Decoder in number mode so it never coerces large integers. Callers that
// already reject UTF-8/BOM/empty input should do so before calling this.
func ScanStructure(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := scanValue(decoder, "$"); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing json value")
		}
		return fmt.Errorf("trailing data: %w", err)
	}
	return nil
}

func scanValue(decoder *json.Decoder, path string) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("invalid value at %s: %w", path, err)
	}

	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("invalid object key at %s: %w", path, err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("invalid object key at %s", path)
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate object key %q at %s", key, path)
			}
			seen[key] = struct{}{}
			if err := scanValue(decoder, childPath(path, key)); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("invalid object at %s: %w", path, err)
		}
		if closing != json.Delim('}') {
			return fmt.Errorf("invalid object closing delimiter at %s", path)
		}
	case '[':
		index := 0
		for decoder.More() {
			if err := scanValue(decoder, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
			index++
		}
		closing, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("invalid array at %s: %w", path, err)
		}
		if closing != json.Delim(']') {
			return fmt.Errorf("invalid array closing delimiter at %s", path)
		}
	default:
		return fmt.Errorf("unexpected delimiter %q at %s", delimiter, path)
	}
	return nil
}

// childPath renders a JSON-pointer-ish location for diagnostics. Simple
// identifiers use dot notation; everything else (empty, special characters,
// array indices handled by the caller) uses a quoted bracket form so the path
// stays unambiguous.
func childPath(parent, key string) string {
	if key == "" {
		return parent + "[\"\"]"
	}
	for _, character := range key {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' {
			return fmt.Sprintf("%s[%q]", parent, key)
		}
	}
	return parent + "." + key
}
