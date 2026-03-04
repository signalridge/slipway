package model

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var slugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

const slugCollisionMaxAttempts = 10000

func NewRequestID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return strings.ToLower(id.String()), nil
}

func IsUUIDv7(raw string) bool {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Version() == 7 && parsed.String() == strings.ToLower(raw)
}

func SlugifyTitle(title string) string {
	slug := strings.ToLower(strings.TrimSpace(title))
	slug = slugSanitizer.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "change"
	}
	return slug
}

func ResolveSlugCollision(baseSlug string, exists func(string) bool) string {
	if baseSlug == "" {
		baseSlug = "change"
	}
	if !exists(baseSlug) {
		return baseSlug
	}
	for n := 2; n <= slugCollisionMaxAttempts; n++ {
		candidate := fmt.Sprintf("%s-%d", baseSlug, n)
		if !exists(candidate) {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", baseSlug, slugCollisionMaxAttempts+1)
}
