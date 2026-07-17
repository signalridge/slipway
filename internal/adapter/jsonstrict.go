package adapter

import "github.com/signalridge/slipway/internal/jsonstrict"

// rejectDuplicateJSONKeys scans one JSON value and rejects duplicate object
// keys, structurally invalid JSON, and trailing values/data. It is a structural
// scan only; callers decode into a typed target separately and compose
// DisallowUnknownFields on top. UTF-8 acceptance remains unchanged from the
// ownership manifest decoder and is intentionally not added by this helper.
func rejectDuplicateJSONKeys(raw []byte) error {
	return jsonstrict.ScanStructure(raw)
}
