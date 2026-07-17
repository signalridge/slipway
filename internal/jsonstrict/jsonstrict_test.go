package jsonstrict_test

import (
	"testing"

	"github.com/signalridge/slipway/internal/jsonstrict"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanStructureAcceptsCanonicalJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
	}{
		{name: "scalar", raw: `42`},
		{name: "string", raw: `"alpha"`},
		{name: "flat object", raw: `{"a":1,"b":2}`},
		{name: "nested object", raw: `{"a":{"b":{"c":1}}}`},
		{name: "array", raw: `[1,2,3]`},
		{name: "mixed", raw: `{"name":"x","nested":{"value":7},"items":[{"id":"one"}]}`},
		{name: "empty object", raw: `{}`},
		{name: "empty array", raw: `[]`},
		{name: "large integer", raw: `9007199254740993`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, jsonstrict.ScanStructure([]byte(test.raw)))
		})
	}
}

func TestScanStructureRejectsNonCanonicalJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "top level duplicate key", raw: `{"a":1,"a":2}`, want: "duplicate object key"},
		{name: "nested duplicate key", raw: `{"a":{"b":1,"b":2}}`, want: "duplicate object key"},
		{name: "array object duplicate key", raw: `[{"id":1,"id":2}]`, want: "duplicate object key"},
		{name: "trailing value", raw: `{"a":1} {}`, want: "trailing json value"},
		{name: "trailing non json data", raw: `{"a":1} nope`, want: "trailing data"},
		{name: "unclosed object", raw: `{"a":1`, want: "invalid object"},
		{name: "malformed value", raw: `tru`, want: "invalid value"},
		{name: "empty input", raw: ``, want: "invalid value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := jsonstrict.ScanStructure([]byte(test.raw))
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestScanStructureFormatsSpecialKeysUnambiguously(t *testing.T) {
	t.Parallel()
	err := jsonstrict.ScanStructure([]byte(`{"a.b":{"id":1,"id":2}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `$["a.b"]`)
}
