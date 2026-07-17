package fsutil

import (
	"fmt"
	"io/fs"
	"os"
	"sort"
	"time"
)

// TopLevelMetadata is metadata for one direct child of a directory. The child
// itself is never opened or followed.
type TopLevelMetadata struct {
	Name    string
	Mode    fs.FileMode
	Size    int64
	ModTime time.Time
}

// LstatTopLevel returns deterministic metadata for the direct children of
// directory. It opens only directory itself and uses Lstat for each exact child
// name, so symlinks and protected child directories are never traversed.
func LstatTopLevel(directory string) ([]TopLevelMetadata, error) {
	info, err := os.Lstat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return []TopLevelMetadata{}, nil
		}
		return nil, fmt.Errorf("lstat metadata directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("metadata directory is not a real directory")
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, fmt.Errorf("open metadata root: %w", err)
	}
	defer func() { _ = root.Close() }()
	rootInfo, err := root.Lstat(".")
	if err != nil {
		return nil, fmt.Errorf("lstat opened metadata root: %w", err)
	}
	if !os.SameFile(info, rootInfo) {
		return nil, fmt.Errorf("metadata directory changed while opening")
	}

	handle, err := root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("open metadata directory: %w", err)
	}
	names, readErr := handle.Readdirnames(-1)
	closeErr := handle.Close()
	if readErr != nil {
		return nil, fmt.Errorf("list metadata directory: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close metadata directory: %w", closeErr)
	}
	sort.Strings(names)

	entries := make([]TopLevelMetadata, 0, len(names))
	for _, name := range names {
		child, err := root.Lstat(name)
		if err != nil {
			return nil, fmt.Errorf("lstat metadata entry %q: %w", name, err)
		}
		entries = append(entries, TopLevelMetadata{
			Name: name, Mode: child.Mode(), Size: child.Size(), ModTime: child.ModTime(),
		})
	}
	return entries, nil
}
