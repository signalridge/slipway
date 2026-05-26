package stringutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnique(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"dedup with preserved order", []string{"b", "a", "b", "c", "a"}, []string{"b", "a", "c"}},
		{"nil input", nil, nil},
		{"empty input", []string{}, nil},
		{"whitespace preserved", []string{" a ", "a", " a "}, []string{" a ", "a"}},
		{"single element", []string{"x"}, []string{"x"}},
		{"all same", []string{"a", "a", "a"}, []string{"a"}},
		{"empty strings kept", []string{"", "a", ""}, []string{"", "a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Unique(tt.in))
		})
	}
}

func TestUniqueSorted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"dedup trim and sort", []string{" b ", "a", "b", " ", "c", "a"}, []string{"a", "b", "c"}},
		{"nil input", nil, nil},
		{"empty input", []string{}, nil},
		{"all whitespace dropped", []string{"  ", "\t", ""}, nil},
		{"single element", []string{" x "}, []string{"x"}},
		{"all same after trim", []string{" a", "a ", " a "}, []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, UniqueSorted(tt.in))
		})
	}
}
