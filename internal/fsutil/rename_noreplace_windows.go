//go:build windows

package fsutil

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsFileRenameInformation struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

func renameNoReplaceAt(oldDirectory *os.File, oldName string, newDirectory *os.File, newName string) error {
	if oldName == "" || newName == "" {
		return errors.New("rename without replacement requires nonempty names")
	}

	objectName, err := windows.NewNTUnicodeString(oldName)
	if err != nil {
		return fmt.Errorf("encode rename source %s: %w", oldName, err)
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(oldDirectory.Fd()),
		ObjectName:    objectName,
	}
	var source windows.Handle
	var status windows.IO_STATUS_BLOCK
	var allocationSize int64
	err = windows.NtCreateFile(
		&source,
		windows.DELETE|windows.SYNCHRONIZE,
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
	runtime.KeepAlive(oldDirectory)
	if err != nil {
		return fmt.Errorf("open rename source %s: %w", oldName, windowsNTError(err))
	}
	defer windows.CloseHandle(source)

	encodedName, err := windows.UTF16FromString(newName)
	if err != nil {
		return fmt.Errorf("encode rename destination %s: %w", newName, err)
	}
	encodedName = encodedName[:len(encodedName)-1]
	var header windowsFileRenameInformation
	headerSize := int(unsafe.Offsetof(header.FileName))
	buffer := make([]byte, headerSize+len(encodedName)*2)
	information := (*windowsFileRenameInformation)(unsafe.Pointer(&buffer[0]))
	information.RootDirectory = windows.Handle(newDirectory.Fd())
	information.FileNameLength = uint32(len(encodedName) * 2)
	copy(unsafe.Slice(&information.FileName[0], len(encodedName)), encodedName)

	err = windows.NtSetInformationFile(
		source,
		&status,
		&buffer[0],
		uint32(len(buffer)),
		windows.FileRenameInformation,
	)
	runtime.KeepAlive(newDirectory)
	if err != nil {
		err = windowsNTError(err)
		if errors.Is(err, windows.ERROR_INVALID_FUNCTION) ||
			errors.Is(err, windows.ERROR_NOT_SUPPORTED) ||
			errors.Is(err, windows.ERROR_INVALID_PARAMETER) ||
			errors.Is(err, windows.ERROR_CALL_NOT_IMPLEMENTED) {
			return fmt.Errorf("%w: rename %s to %s: %v", ErrFileTransactionNoReplaceUnsupported, oldName, newName, err)
		}
		return fmt.Errorf("rename %s to %s without replacement: %w", oldName, newName, err)
	}
	return nil
}

func windowsNTError(err error) error {
	if status, ok := err.(windows.NTStatus); ok {
		return status.Errno()
	}
	return err
}
