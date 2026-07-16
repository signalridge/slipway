package runstore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/signalridge/slipway/internal/fsutil"
)

const (
	rootRenameAttempts = 10
)

type leafIdentity struct {
	exists bool
	info   os.FileInfo
}

func restrictPrivateFile(file *os.File, perm os.FileMode) error {
	if perm.Perm()&0o077 != 0 {
		return nil
	}
	if err := fsutil.RestrictToOwner(file); err != nil {
		return fmt.Errorf("restrict private file to owner: %w", err)
	}
	return nil
}

func openRegularFileInRoot(root *os.Root, name string, flags int, perm os.FileMode, create bool) (*os.File, bool, error) {
	if name == "" || filepath.Base(name) != name {
		return nil, false, fmt.Errorf("invalid leaf path %q", name)
	}
	for attempt := 0; attempt < 3; attempt++ {
		before, err := root.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) {
			if !create {
				return nil, false, err
			}
			file, openErr := root.OpenFile(name, flags|os.O_CREATE|os.O_EXCL, perm)
			if errors.Is(openErr, fs.ErrExist) {
				continue
			}
			if openErr != nil {
				return nil, false, openErr
			}
			if err := verifyOpenedRegularFileInRoot(root, name, file); err != nil {
				_ = file.Close()
				return nil, false, err
			}
			if err := restrictPrivateFile(file, perm); err != nil {
				_ = file.Close()
				return nil, false, err
			}
			return file, true, nil
		}
		if err != nil {
			return nil, false, err
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
			return nil, false, fmt.Errorf("path %q is not a regular file", name)
		}
		file, openErr := root.OpenFile(name, flags, perm)
		if errors.Is(openErr, fs.ErrNotExist) {
			continue
		}
		if openErr != nil {
			return nil, false, openErr
		}
		if err := verifyOpenedRegularFileInRoot(root, name, file); err != nil {
			_ = file.Close()
			return nil, false, err
		}
		opened, statErr := file.Stat()
		if statErr != nil || !os.SameFile(before, opened) {
			_ = file.Close()
			if statErr != nil {
				return nil, false, statErr
			}
			continue
		}
		return file, false, nil
	}
	return nil, false, fmt.Errorf("path %q changed while opening", name)
}

func verifyOpenedRegularFileInRoot(root *os.Root, name string, file *os.File) error {
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	current, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if current.Mode()&os.ModeSymlink != 0 || !current.Mode().IsRegular() || !opened.Mode().IsRegular() || !os.SameFile(opened, current) {
		return fmt.Errorf("path %q changed or is not a regular file", name)
	}
	return nil
}

func inspectRegularFileOrMissingInRoot(root *os.Root, name string) (leafIdentity, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return leafIdentity{}, nil
	}
	if err != nil {
		return leafIdentity{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return leafIdentity{}, fmt.Errorf("path %q is not a regular file", name)
	}
	return leafIdentity{exists: true, info: info}, nil
}

func pinRegularFileOrMissingInRoot(root *os.Root, name string) (leafIdentity, *os.File, error) {
	for attempt := 0; attempt < 3; attempt++ {
		file, _, err := openRegularFileInRoot(root, name, os.O_RDONLY, 0, false)
		if errors.Is(err, fs.ErrNotExist) {
			current, inspectErr := inspectRegularFileOrMissingInRoot(root, name)
			if inspectErr != nil {
				return leafIdentity{}, nil, inspectErr
			}
			if !current.exists {
				return leafIdentity{}, nil, nil
			}
			continue
		}
		if err != nil {
			return leafIdentity{}, nil, err
		}
		info, statErr := file.Stat()
		if statErr != nil {
			_ = file.Close()
			return leafIdentity{}, nil, statErr
		}
		return leafIdentity{exists: true, info: info}, file, nil
	}
	return leafIdentity{}, nil, fmt.Errorf("path %q changed while pinning", name)
}

func verifyPinnedLeafIdentity(root *os.Root, name string, expected leafIdentity, pinned *os.File) error {
	if !expected.exists {
		if pinned != nil {
			return fmt.Errorf("path %q has a pinned handle but no expected identity", name)
		}
		return verifyLeafIdentity(root, name, expected)
	}
	if pinned == nil {
		return fmt.Errorf("path %q expected identity has no pinned handle", name)
	}
	opened, err := pinned.Stat()
	if err != nil {
		return fmt.Errorf("stat pinned path %q: %w", name, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(opened, expected.info) {
		return fmt.Errorf("path %q no longer matches its pinned regular file", name)
	}
	current, err := inspectRegularFileOrMissingInRoot(root, name)
	if err != nil {
		return err
	}
	if !current.exists || !os.SameFile(opened, current.info) {
		return fmt.Errorf("path %q identity changed", name)
	}
	return nil
}

func verifyLeafIdentity(root *os.Root, name string, expected leafIdentity) error {
	current, err := inspectRegularFileOrMissingInRoot(root, name)
	if err != nil {
		return err
	}
	if current.exists != expected.exists {
		return fmt.Errorf("path %q existence changed", name)
	}
	if current.exists && !os.SameFile(current.info, expected.info) {
		return fmt.Errorf("path %q identity changed", name)
	}
	return nil
}

func randomRunLeaf(prefix, destination string) (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return prefix + destination + "-" + hex.EncodeToString(random[:]), nil
}

func createTemporaryFileInRoot(root *os.Root, destination string, perm os.FileMode) (string, *os.File, error) {
	for attempt := 0; attempt < rootRenameAttempts; attempt++ {
		name, err := randomRunLeaf(".tmp-", destination)
		if err != nil {
			return "", nil, err
		}
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, perm)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		if err := verifyOpenedRegularFileInRoot(root, name, file); err != nil {
			_ = file.Close()
			return "", nil, err
		}
		if err := restrictPrivateFile(file, perm); err != nil {
			_ = file.Close()
			_ = root.Remove(name)
			return "", nil, err
		}
		return name, file, nil
	}
	return "", nil, errors.New("could not allocate journal temp file")
}
