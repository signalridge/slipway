package cmd

import "testing"

import "github.com/stretchr/testify/assert"

func TestResolveInputContextPath(t *testing.T) {
	t.Parallel()

	t.Run("empty target returns empty", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", resolveInputContextPath("/project", "/workspace", ""))
	})

	t.Run("absolute target is preserved", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "/tmp/bundle", resolveInputContextPath("/project", "/workspace", "/tmp/bundle"))
	})

	t.Run("relative target prefers workspace root", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "/workspace/artifacts/changes/demo", resolveInputContextPath("/project", "/workspace", "artifacts/changes/demo"))
	})

	t.Run("relative target falls back to project root", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "/project/artifacts/changes/demo", resolveInputContextPath("/project", "", "artifacts/changes/demo"))
	})
}
