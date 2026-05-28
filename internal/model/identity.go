package model

import (
	"regexp"
	"strings"
)

var slugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

const MaxSlugLength = 60

func SlugifyTitle(title string) string {
	slug := strings.ToLower(strings.TrimSpace(title))
	slug = slugSanitizer.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "change"
	}
	if len(slug) > MaxSlugLength {
		slug = strings.Trim(slug[:MaxSlugLength], "-")
		if slug == "" {
			return "change"
		}
	}
	return slug
}
