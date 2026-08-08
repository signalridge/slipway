package adapter

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// unownedCapabilityDetail names the generated capability files that survive
// without a current ownership manifest, so a warning about a missing manifest
// points at the files it is actually about.
//
// Several adapters write their capability files outside their ownership root:
// copilot owns `.github/copilot` but writes `.github/agents`, and kilo owns
// `.kilocode` but writes launchers under `.kilo`. Deleting the ownership root
// by hand therefore takes the manifest and the marker with it and leaves those
// files behind, where uninstall cannot remove them and refresh never adopts
// them. Only paths this version would itself generate are reported, so a file
// the user wrote in a shared directory is never implicated, and nothing is
// deleted, migrated, or adopted.
func unownedCapabilityDetail(root string, host Host) string {
	orphans := unownedCapabilityFiles(root, host)
	if len(orphans) == 0 {
		return ""
	}
	return fmt.Sprintf(
		"detected, current ownership manifest is missing, and %d generated capability file(s) remain unowned: %s. Uninstall cannot remove them and install --refresh does not adopt them. Back up and inspect them, remove the ones you want Slipway to recreate, then run slipway install %s",
		len(orphans),
		strings.Join(orphans, ", "),
		installInstruction(host),
	)
}

func unownedCapabilityFiles(root string, host Host) []string {
	seen := map[string]bool{}
	for _, surface := range candidateSurfaceKinds(host) {
		probe := host
		probe.SurfaceKind = surface
		files, err := generateHostFiles(probe)
		if err != nil {
			continue
		}
		for _, file := range files {
			if seen[file.Relative] {
				continue
			}
			path, err := safePath(root, file.Relative, "")
			if err != nil {
				continue
			}
			if _, err := os.Lstat(path); err == nil {
				seen[file.Relative] = true
			}
		}
	}
	orphans := make([]string, 0, len(seen))
	for relative := range seen {
		orphans = append(orphans, relative)
	}
	sort.Strings(orphans)
	return orphans
}

// candidateSurfaceKinds lists the surfaces a host can generate for. Only kiro
// has more than one, and without a manifest neither is known, so both are
// probed rather than guessed.
func candidateSurfaceKinds(host Host) []string {
	if host.SurfaceKind != "" {
		return []string{host.SurfaceKind}
	}
	kinds := make([]string, 0, 2)
	for _, surface := range []string{"ide", "cli"} {
		if kind, valid := selectedSurfaceKind(surface); valid {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

func installInstruction(host Host) string {
	if host.ID == "kiro" {
		return "--tool kiro --surface ide or slipway install --tool kiro --surface cli"
	}
	return "--tool " + host.ID
}
