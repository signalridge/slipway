package runstore

import (
	"errors"
	"unicode/utf8"

	"github.com/signalridge/slipway/internal/jsonstrict"
)

// validateJournalJSONStructure rejects structurally invalid journal JSON:
// non-UTF-8 input, duplicate object keys, and trailing values/data. It is a
// structural scan only; callers decode into a typed target separately and
// compose DisallowUnknownFields on top.
func validateJournalJSONStructure(raw []byte) error {
	if !utf8.Valid(raw) {
		return errors.New("input is not valid utf-8")
	}
	return jsonstrict.ScanStructure(raw)
}
