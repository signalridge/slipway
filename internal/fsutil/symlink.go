package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrSymlinkTargetOutsideRoot = errors.New("symlink target resolves outside allowed root")

// ResolveSymlinkTarget resolves linkTarget relative to linkPath and returns the
// target's real path.
func ResolveSymlinkTarget(linkPath, linkTarget string) (string, os.FileInfo, error) {
	target := linkTarget
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(linkPath), target)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", nil, err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", nil, err
	}
	resolved = filepath.Clean(resolved)
	info, err := os.Stat(resolved)
	if err != nil {
		return "", nil, err
	}
	return resolved, info, nil
}

// ResolveSymlinkTargetWithin resolves linkTarget relative to linkPath and
// returns the target's real path only when it stays within allowedRoot.
func ResolveSymlinkTargetWithin(allowedRoot, linkPath, linkTarget string) (string, os.FileInfo, error) {
	root, err := realExistingPath(allowedRoot)
	if err != nil {
		return "", nil, err
	}
	resolved, info, err := ResolveSymlinkTarget(linkPath, linkTarget)
	if err != nil {
		return "", nil, err
	}
	if !PathWithin(root, resolved) {
		return "", nil, fmt.Errorf("%w: %q outside %q", ErrSymlinkTargetOutsideRoot, resolved, root)
	}
	return resolved, info, nil
}

// PathWithin reports whether path is equal to root or contained by root after
// both paths are made absolute and clean.
func PathWithin(root, path string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(rootAbs), filepath.Clean(pathAbs))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func realExistingPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
