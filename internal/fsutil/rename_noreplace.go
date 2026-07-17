package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// RenameNoReplace atomically renames one direct child of root to another
// without replacing an existing destination.
func RenameNoReplace(root *os.Root, oldName, newName string) error {
	if root == nil {
		return errors.New("rename without replacement: nil root")
	}
	if oldName == "" || filepath.Base(oldName) != oldName || newName == "" || filepath.Base(newName) != newName {
		return fmt.Errorf("rename without replacement requires direct child names: %q to %q", oldName, newName)
	}
	return renameNoReplaceRoots(root, root, oldName, newName)
}
