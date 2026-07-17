//go:build windows

package fsutil

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	localSystemSID     = "S-1-5-18"
	extendedFileIDType = 2
)

var (
	procOpenFileByID = windows.NewLazySystemDLL("kernel32.dll").NewProc("OpenFileById")
	procReOpenFile   = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReOpenFile")
)

type fileIDInfo struct {
	volumeSerialNumber uint64
	fileID             [16]byte
}

type fileIDDescriptor struct {
	size   uint32
	typeID uint32
	fileID [16]byte
}

// RestrictToOwner applies a protected DACL that grants full control only to
// the current user and LocalSystem.
func RestrictToOwner(file *os.File) error {
	return restrictToOwner(file)
}

func restrictToOwner(file *os.File) error {
	if file == nil {
		return errors.New("restrict owner access: nil file")
	}
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect ACL target: %w", err)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return errors.New("ACL target is not a regular file or directory")
	}

	handle, err := reopenFileHandle(file, windows.READ_CONTROL|windows.WRITE_DAC)
	if err != nil {
		return fmt.Errorf("reopen ACL target handle: %w", err)
	}
	defer windows.CloseHandle(handle)

	userSID, systemSID, err := ownerOnlySIDs()
	if err != nil {
		return err
	}
	inheritance := uint32(windows.NO_INHERITANCE)
	if info.IsDir() {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	entries := []windows.EXPLICIT_ACCESS{ownerOnlyAccessEntry(userSID, windows.TRUSTEE_IS_USER, inheritance)}
	if !userSID.Equals(systemSID) {
		entries = append(entries, ownerOnlyAccessEntry(systemSID, windows.TRUSTEE_IS_WELL_KNOWN_GROUP, inheritance))
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return fmt.Errorf("build owner-only DACL: %w", err)
	}
	if err := windows.SetSecurityInfo(
		handle,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	); err != nil {
		return fmt.Errorf("set owner-only DACL: %w", err)
	}
	private, err := ownerACLHandleIsPrivate(handle, info.IsDir())
	if err != nil {
		return fmt.Errorf("verify owner-only DACL: %w", err)
	}
	if !private {
		return errors.New("verify owner-only DACL: target retained unexpected access")
	}

	// The identity-bound reopen performs a fresh access check, so pathname
	// replacement cannot satisfy the effective-access verification.
	access, err := reopenFileHandle(file, windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE|windows.READ_CONTROL|windows.WRITE_DAC)
	if err != nil {
		return fmt.Errorf("verify effective owner access: %w", err)
	}
	return windows.CloseHandle(access)
}

func ownerOnlyAccessEntry(sid *windows.SID, trusteeType windows.TRUSTEE_TYPE, inheritance uint32) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  trusteeType,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}

func ownerOnlySIDs() (*windows.SID, *windows.SID, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, nil, fmt.Errorf("query current Windows user SID: %w", err)
	}
	if user == nil || user.User.Sid == nil {
		return nil, nil, fmt.Errorf("query current Windows user SID: token returned no SID")
	}
	system, err := windows.StringToSid(localSystemSID)
	if err != nil {
		return nil, nil, fmt.Errorf("parse LocalSystem SID: %w", err)
	}
	return user.User.Sid, system, nil
}

func ownerACLIsPrivate(file *os.File) (bool, error) {
	if file == nil {
		return false, errors.New("verify owner ACL: nil file")
	}
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	handle, err := reopenFileHandle(file, windows.READ_CONTROL)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(handle)
	return ownerACLHandleIsPrivate(handle, info.IsDir())
}

func reopenFileHandle(file *os.File, access uint32) (windows.Handle, error) {
	handle, err := reopenWithReOpenFile(file, access)
	if err == nil {
		return handle, nil
	}
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return 0, err
	}

	// ReOpenFile can deny WRITE_DAC when the original directory handle came
	// from os.Root.Open and therefore only carries FILE_GENERIC_READ. Open by
	// the already-open object's file ID to perform a fresh access check without
	// resolving a pathname or losing the object binding.
	handle, byIDErr := reopenByFileID(file, access)
	if byIDErr != nil {
		return 0, errors.Join(err, fmt.Errorf("reopen by file ID: %w", byIDErr))
	}
	return handle, nil
}

func reopenWithReOpenFile(file *os.File, access uint32) (windows.Handle, error) {
	handleValue, _, callErr := procReOpenFile.Call(
		file.Fd(),
		uintptr(access),
		uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE),
		uintptr(windows.FILE_FLAG_BACKUP_SEMANTICS),
	)
	runtime.KeepAlive(file)
	return checkedWindowsHandle(handleValue, callErr)
}

func reopenByFileID(file *os.File, access uint32) (windows.Handle, error) {
	var info fileIDInfo
	if err := windows.GetFileInformationByHandleEx(
		windows.Handle(file.Fd()),
		windows.FileIdInfo,
		(*byte)(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		runtime.KeepAlive(file)
		return 0, fmt.Errorf("query file ID: %w", err)
	}

	descriptor := fileIDDescriptor{
		size:   uint32(unsafe.Sizeof(fileIDDescriptor{})),
		typeID: extendedFileIDType,
		fileID: info.fileID,
	}
	handleValue, _, callErr := procOpenFileByID.Call(
		file.Fd(),
		uintptr(unsafe.Pointer(&descriptor)),
		uintptr(access),
		uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE),
		0,
		uintptr(windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT),
	)
	runtime.KeepAlive(file)
	runtime.KeepAlive(&descriptor)
	return checkedWindowsHandle(handleValue, callErr)
}

func checkedWindowsHandle(handleValue uintptr, callErr error) (windows.Handle, error) {
	handle := windows.Handle(handleValue)
	if handle != windows.InvalidHandle {
		return handle, nil
	}
	if callErr == syscall.Errno(0) {
		callErr = windows.ERROR_INVALID_HANDLE
	}
	return 0, callErr
}

func ownerACLHandleIsPrivate(handle windows.Handle, directory bool) (bool, error) {
	descriptor, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false, err
	}
	if descriptor == nil {
		return false, nil
	}
	defer runtime.KeepAlive(descriptor)

	control, _, err := descriptor.Control()
	if err != nil {
		return false, err
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return false, nil
	}
	dacl, defaulted, err := descriptor.DACL()
	if err != nil {
		return false, err
	}
	if dacl == nil || defaulted {
		return false, nil
	}

	userSID, systemSID, err := ownerOnlySIDs()
	if err != nil {
		return false, err
	}
	trusteeCount := 2
	if userSID.Equals(systemSID) {
		trusteeCount = 1
	}
	if int(dacl.AceCount) < trusteeCount || int(dacl.AceCount) > trusteeCount*2 {
		return false, nil
	}

	type aceState struct {
		effective   bool
		inheritable bool
	}
	var user, system aceState
	const allowedFlags = uint8(
		windows.OBJECT_INHERIT_ACE |
			windows.CONTAINER_INHERIT_ACE |
			windows.INHERIT_ONLY_ACE,
	)
	const inheritanceFlags = uint8(windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE)
	for index := uint16(0); index < dacl.AceCount; index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(index), &ace); err != nil {
			return false, err
		}
		if ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE ||
			ace.Header.AceFlags&^allowedFlags != 0 ||
			ace.Header.AceSize < uint16(unsafe.Offsetof(ace.SidStart)+4) ||
			!aceGrantsFullControl(ace.Mask) {
			return false, nil
		}

		flags := ace.Header.AceFlags
		inherits := flags&inheritanceFlags != 0
		inheritOnly := flags&windows.INHERIT_ONLY_ACE != 0
		if (!directory && flags != 0) || (inheritOnly && !inherits) {
			return false, nil
		}

		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		state := &user
		switch {
		case userSID.Equals(aceSID):
		case !userSID.Equals(systemSID) && systemSID.Equals(aceSID):
			state = &system
		default:
			return false, nil
		}
		if !inheritOnly {
			if state.effective {
				return false, nil
			}
			state.effective = true
		}
		if inherits {
			if state.inheritable {
				return false, nil
			}
			state.inheritable = true
		}
	}

	complete := func(state aceState) bool {
		return state.effective && (!directory || state.inheritable)
	}
	if userSID.Equals(systemSID) {
		return complete(user), nil
	}
	return complete(user) && complete(system), nil
}

func aceGrantsFullControl(mask windows.ACCESS_MASK) bool {
	if mask&windows.GENERIC_ALL != 0 {
		return true
	}
	required := windows.ACCESS_MASK(
		windows.FILE_GENERIC_READ |
			windows.FILE_GENERIC_WRITE |
			windows.FILE_GENERIC_EXECUTE |
			windows.DELETE |
			windows.WRITE_DAC |
			windows.WRITE_OWNER,
	)
	return mask&required == required
}

func ownerProtectionIsPrivate(file *os.File, _ os.FileMode) bool {
	private, err := ownerACLIsPrivate(file)
	return err == nil && private
}
