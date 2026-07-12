package autopilot

import (
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type strictJSONTestValue struct {
	Name   string `json:"name"`
	Nested struct {
		Value int `json:"value"`
	} `json:"nested"`
	Items []struct {
		ID string `json:"id"`
	} `json:"items"`
}

func TestDecodeStrictJSONAcceptsOneExactValue(t *testing.T) {
	t.Parallel()

	var decoded strictJSONTestValue
	err := decodeStrictJSON([]byte(`{"name":"alpha","nested":{"value":7},"items":[{"id":"one"}]}`), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "alpha", decoded.Name)
	assert.Equal(t, 7, decoded.Nested.Value)
	require.Len(t, decoded.Items, 1)
	assert.Equal(t, "one", decoded.Items[0].ID)
}

func TestDecodeStrictJSONRejectsNonCanonicalInputs(t *testing.T) {
	t.Parallel()

	valid := `{"name":"alpha","nested":{"value":7},"items":[{"id":"one"}]}`
	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{name: "top level duplicate key", raw: []byte(`{"name":"alpha","name":"beta","nested":{"value":7},"items":[]}`), want: "duplicate object key"},
		{name: "nested duplicate key", raw: []byte(`{"name":"alpha","nested":{"value":7,"value":8},"items":[]}`), want: "duplicate object key"},
		{name: "array object duplicate key", raw: []byte(`{"name":"alpha","nested":{"value":7},"items":[{"id":"one","id":"two"}]}`), want: "duplicate object key"},
		{name: "top level unknown field", raw: []byte(`{"name":"alpha","nested":{"value":7},"items":[],"future":true}`), want: "unknown field"},
		{name: "nested unknown field", raw: []byte(`{"name":"alpha","nested":{"value":7,"future":true},"items":[]}`), want: "unknown field"},
		{name: "case variant unknown field", raw: []byte(`{"Name":"alpha","nested":{"value":7},"items":[]}`), want: "unknown field"},
		{name: "wrong scalar type", raw: []byte(`{"name":"alpha","nested":{"value":"seven"},"items":[]}`), want: "cannot unmarshal"},
		{name: "null scalar", raw: []byte(`{"name":"alpha","nested":{"value":null},"items":[]}`), want: "null is not allowed"},
		{name: "trailing value", raw: []byte(valid + ` {}`), want: "trailing json value"},
		{name: "trailing non json data", raw: []byte(valid + ` nope`), want: "trailing data"},
		{name: "invalid utf8", raw: append([]byte(`{"name":"`), append([]byte{0xff}, []byte(`","nested":{"value":7},"items":[]}`)...)...), want: "valid utf-8"},
		{name: "utf8 bom", raw: append([]byte{0xef, 0xbb, 0xbf}, []byte(valid)...), want: "bom"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var decoded strictJSONTestValue
			err := decodeStrictJSON(test.raw, &decoded)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
			first, _ := utf8.DecodeRuneInString(err.Error())
			assert.True(t, unicode.IsLower(first), "error must begin with lower-case context: %q", err)
		})
	}
}
