package adapter

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// legacyV1PathInventory is the exact file inventory emitted into ownership
// manifests by the final governance release. Version 1 manifests are accepted
// only as read-only deletion proof during the breaking cutover.
//
//go:embed legacy_v1_paths.txt
var legacyV1PathInventory string

var legacyV1Claims = sync.OnceValue(func() map[string]struct{} {
	claims := make(map[string]struct{})
	for _, line := range strings.Split(legacyV1PathInventory, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		hostID, relative, found := strings.Cut(line, "\t")
		if !found || hostID == "" || relative == "" {
			panic("invalid embedded legacy ownership inventory")
		}
		claims[hostID+"\x00"+relative] = struct{}{}
	}
	return claims
})

func legacyV1ClaimAllowed(host Host, relative string) bool {
	_, allowed := legacyV1Claims()[host.ID+"\x00"+relative]
	return allowed
}

func currentClaimAllowed(host Host, relative string) (bool, error) {
	files, err := generateHostFiles(host)
	if err != nil {
		return false, fmt.Errorf("enumerate managed files for %s: %w", host.ID, err)
	}
	for _, file := range files {
		if file.Relative == relative {
			return true, nil
		}
	}
	return false, nil
}

func sentinelRelative(host Host) string {
	return filepath.ToSlash(filepath.Join(host.OwnershipRoot, "slipway", sentinelFileName))
}

func isLegacySentinelClaim(host Host, relative string) bool {
	return relative == sentinelRelative(host)
}
