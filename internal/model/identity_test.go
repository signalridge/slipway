package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugifyTitleLimitsLongSlugs(t *testing.T) {
	t.Parallel()

	slug := SlugifyTitle(strings.Repeat("long ", 80))

	assert.LessOrEqual(t, len(slug), MaxSlugLength)
	assert.False(t, strings.HasSuffix(slug, "-"))
	assert.NotEmpty(t, slug)
}

func TestSlugifyTitleFallsBackWhenInputHasNoSlugCharacters(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "change", SlugifyTitle("!!!"))
}
