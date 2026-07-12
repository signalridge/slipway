//go:build windows

package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func openSymlinkIdentity(root *os.Root, name string) (*os.File, error) {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return nil, fmt.Errorf("symlink identity requires one leaf name: %q", name)
	}

	directory, err := root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("open symlink identity parent: %w", err)
	}
	defer directory.Close()

	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, fmt.Errorf("encode symlink identity %s: %w", name, err)
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(directory.Fd()),
		ObjectName:    objectName,
	}
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	var allocationSize int64
	err = windows.NtCreateFile(
		&handle,
		windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		attributes,
		&status,
		&allocationSize,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_FOR_BACKUP_INTENT|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0,
		0,
	)
	runtime.KeepAlive(directory)
	if err != nil {
		return nil, fmt.Errorf("open symlink identity %s: %w", name, windowsNTError(err))
	}
	identity := os.NewFile(uintptr(handle), name)
	if identity == nil {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("open symlink identity %s: invalid handle", name)
	}
	if err := validateOpenedSymlinkIdentity(root, name, identity); err != nil {
		_ = identity.Close()
		return nil, err
	}
	return identity, nil
}

func validateOpenedSymlinkIdentity(root *os.Root, name string, identity *os.File) error {
	opened, err := identity.Stat()
	if err != nil {
		return fmt.Errorf("stat symlink identity %s: %w", name, err)
	}
	current, err := root.Lstat(name)
	if err != nil {
		return fmt.Errorf("inspect symlink identity %s: %w", name, err)
	}
	if opened.Mode()&os.ModeSymlink == 0 || current.Mode()&os.ModeSymlink == 0 || !os.SameFile(opened, current) {
		return fmt.Errorf("symlink identity %s changed while opening", name)
	}
	return nil
}

type identityReparseDataBuffer struct {
	ReparseTag        uint32
	ReparseDataLength uint16
	Reserved          uint16
	ReparseBuffer     byte
}

type identitySymlinkReparseBuffer struct {
	SubstituteNameOffset uint16
	SubstituteNameLength uint16
	PrintNameOffset      uint16
	PrintNameLength      uint16
	Flags                uint32
	PathBuffer           [1]uint16
}

func readSymlinkIdentity(identity *os.File) (string, error) {
	buffer := make([]byte, windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)
	var bytesReturned uint32
	err := windows.DeviceIoControl(
		windows.Handle(identity.Fd()),
		windows.FSCTL_GET_REPARSE_POINT,
		nil,
		0,
		&buffer[0],
		uint32(len(buffer)),
		&bytesReturned,
		nil,
	)
	runtime.KeepAlive(identity)
	if err != nil {
		return "", fmt.Errorf("read pinned symlink: %w", err)
	}
	return decodeSymlinkReparseData(buffer, bytesReturned)
}

func decodeSymlinkReparseData(buffer []byte, bytesReturned uint32) (string, error) {
	headerSize := int(unsafe.Offsetof(identityReparseDataBuffer{}.ReparseBuffer))
	if int(bytesReturned) < headerSize || int(bytesReturned) > len(buffer) {
		return "", fmt.Errorf("read pinned symlink: truncated reparse data")
	}
	reparse := (*identityReparseDataBuffer)(unsafe.Pointer(&buffer[0]))
	declaredEnd := headerSize + int(reparse.ReparseDataLength)
	if declaredEnd < headerSize || declaredEnd > int(bytesReturned) || declaredEnd > len(buffer) {
		return "", fmt.Errorf("read pinned symlink: invalid declared reparse length")
	}
	switch reparse.ReparseTag {
	case windows.IO_REPARSE_TAG_SYMLINK:
		pathOffset := int(unsafe.Offsetof(identitySymlinkReparseBuffer{}.PathBuffer))
		if int(reparse.ReparseDataLength) < pathOffset {
			return "", fmt.Errorf("read pinned symlink: truncated symbolic-link reparse data")
		}
		data := (*identitySymlinkReparseBuffer)(unsafe.Pointer(&reparse.ReparseBuffer))
		const symlinkFlagRelative = 1
		if data.Flags & ^uint32(symlinkFlagRelative) != 0 {
			return "", fmt.Errorf("read pinned symlink: unsupported symbolic-link flags %#x", data.Flags)
		}
		target, err := decodeReparseName(buffer, declaredEnd, pathOffset, data.SubstituteNameOffset, data.SubstituteNameLength)
		if err != nil {
			return "", err
		}
		if data.Flags&symlinkFlagRelative != 0 {
			if filepath.VolumeName(target) != "" {
				return "", fmt.Errorf("read pinned symlink: relative flag has a volume-relative substitute name")
			}
			return target, nil
		}
		target = normalizeWindowsSymlinkTarget(target)
		if filepath.VolumeName(target) == "" || !filepath.IsAbs(target) {
			return "", fmt.Errorf("read pinned symlink: absolute flag has a relative substitute name")
		}
		return target, nil
	case windows.IO_REPARSE_TAG_MOUNT_POINT:
		return "", fmt.Errorf("read pinned symlink: mount-point reparse objects cannot be restored as symbolic links")
	default:
		return "", fmt.Errorf("read pinned symlink: unsupported reparse tag %#x", reparse.ReparseTag)
	}
}

func decodeReparseName(buffer []byte, declaredEnd, relativePathOffset int, nameOffset, nameLength uint16) (string, error) {
	const reparseHeaderSize = 8
	start := reparseHeaderSize + relativePathOffset + int(nameOffset)
	end := start + int(nameLength)
	if nameOffset%2 != 0 || nameLength%2 != 0 || start < reparseHeaderSize+relativePathOffset || end < start || end > declaredEnd || end > len(buffer) {
		return "", fmt.Errorf("read pinned symlink: invalid reparse target bounds")
	}
	if nameLength == 0 {
		return "", fmt.Errorf("read pinned symlink: empty substitute name")
	}
	codeUnits := unsafe.Slice((*uint16)(unsafe.Pointer(&buffer[start])), int(nameLength)/2)
	runes := make([]rune, 0, len(codeUnits))
	for index := 0; index < len(codeUnits); index++ {
		unit := codeUnits[index]
		switch {
		case unit == 0:
			return "", fmt.Errorf("read pinned symlink: substitute name contains NUL")
		case unit >= 0xd800 && unit <= 0xdbff:
			if index+1 >= len(codeUnits) || codeUnits[index+1] < 0xdc00 || codeUnits[index+1] > 0xdfff {
				return "", fmt.Errorf("read pinned symlink: invalid UTF-16 substitute name")
			}
			high := rune(unit - 0xd800)
			low := rune(codeUnits[index+1] - 0xdc00)
			runes = append(runes, 0x10000+(high<<10)+low)
			index++
		case unit >= 0xdc00 && unit <= 0xdfff:
			return "", fmt.Errorf("read pinned symlink: invalid UTF-16 substitute name")
		default:
			runes = append(runes, rune(unit))
		}
	}
	return string(runes), nil
}

func normalizeWindowsSymlinkTarget(target string) string {
	if len(target) < 4 || target[:4] != `\??\` {
		return target
	}
	target = target[4:]
	if len(target) >= 4 && target[:4] == `UNC\` {
		return `\\` + target[4:]
	}
	if len(target) >= 7 && target[:7] == `Volume{` {
		return `\\?\` + target
	}
	return target
}

func validateSymlinkTransactionIdentity(_ os.FileInfo) error {
	// Recreating any Windows symbolic link can require a privilege that was not
	// needed to inspect or move the existing reparse point. Reject the guarded
	// pre-state before the first mutation instead of discovering during rollback
	// that the original object cannot be restored exactly.
	return fmt.Errorf("%w: Windows symbolic links cannot be recreated without relying on symbolic-link creation privilege", ErrFileTransactionSymlinkUnsupported)
}
