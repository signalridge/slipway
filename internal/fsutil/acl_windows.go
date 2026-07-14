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

const localSystemSID = "S-1-5-18"

var procReOpenFile = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReOpenFile")

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
	private, err := ownerACLHandleIsPrivate(handle)
	if err != nil {
		return fmt.Errorf("verify owner-only DACL: %w", err)
	}
	if !private {
		return errors.New("verify owner-only DACL: target retained unexpected access")
	}

	// ReOpenFile performs a fresh access check while remaining bound to this
	// filesystem object, so a pathname replacement cannot satisfy verification.
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
	handle, err := reopenFileHandle(file, windows.READ_CONTROL)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(handle)
	return ownerACLHandleIsPrivate(handle)
}

func reopenFileHandle(file *os.File, access uint32) (windows.Handle, error) {
	handleValue, _, callErr := procReOpenFile.Call(
		file.Fd(),
		uintptr(access),
		uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE),
		uintptr(windows.FILE_FLAG_BACKUP_SEMANTICS),
	)
	runtime.KeepAlive(file)
	handle := windows.Handle(handleValue)
	if handle == windows.InvalidHandle {
		if callErr == syscall.Errno(0) {
			callErr = windows.ERROR_INVALID_HANDLE
		}
		return 0, callErr
	}
	return handle, nil
}

func ownerACLHandleIsPrivate(handle windows.Handle) (bool, error) {
	descriptor, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false, err
	}
	if descriptor == nil {
		return false, nil
	}
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
	expectedEntries := 2
	if userSID.Equals(systemSID) {
		expectedEntries = 1
	}
	if int(dacl.AceCount) != expectedEntries {
		return false, nil
	}
	seenUser := false
	seenSystem := false
	const allowedFlags = uint8(windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE)
	for index := uint16(0); index < dacl.AceCount; index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(index), &ace); err != nil {
			return false, err
		}
		if ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE ||
			ace.Header.AceFlags & ^allowedFlags != 0 ||
			ace.Header.AceSize < uint16(unsafe.Offsetof(ace.SidStart)+4) ||
			!aceGrantsFullControl(ace.Mask) {
			return false, nil
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if userSID.Equals(systemSID) {
			if !userSID.Equals(aceSID) {
				return false, nil
			}
			seenUser, seenSystem = true, true
			continue
		}
		switch {
		case userSID.Equals(aceSID):
			if seenUser {
				return false, nil
			}
			seenUser = true
		case systemSID.Equals(aceSID):
			if seenSystem {
				return false, nil
			}
			seenSystem = true
		default:
			return false, nil
		}
	}
	runtime.KeepAlive(descriptor)
	return seenUser && seenSystem, nil
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
