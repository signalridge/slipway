package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootCommandDoesNotExposeLearn(t *testing.T) {
	t.Parallel()

	for _, child := range newRootCmd().Commands() {
		assert.NotEqual(t, "learn", child.Name())
	}
}
