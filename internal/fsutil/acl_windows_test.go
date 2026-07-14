//go:build windows

package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestRestrictToOwnerSetsProtectedUserAndSystemDACL(t *testing.T) {
	tests := []struct {
		name      string
		directory bool
	}{
		{name: "file"},
		{name: "directory", directory: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "private")
			if test.directory {
				require.NoError(t, os.Mkdir(path, 0o700))
			} else {
				require.NoError(t, os.WriteFile(path, []byte("private"), 0o600))
			}
			require.NoError(t, restrictToOwner(path))

			descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
			require.NoError(t, err)
			control, _, err := descriptor.Control()
			require.NoError(t, err)
			assert.NotZero(t, control&windows.SE_DACL_PROTECTED)
			dacl, defaulted, err := descriptor.DACL()
			require.NoError(t, err)
			assert.False(t, defaulted)

			userSID, systemSID, err := ownerOnlySIDs()
			require.NoError(t, err)
			expectedEntries := 2
			if userSID.Equals(systemSID) {
				expectedEntries = 1
			}
			require.Equal(t, expectedEntries, int(dacl.AceCount))
			seen := make(map[string]bool, expectedEntries)
			for index := uint16(0); index < dacl.AceCount; index++ {
				var ace *windows.ACCESS_ALLOWED_ACE
				require.NoError(t, windows.GetAce(dacl, uint32(index), &ace))
				require.NotNil(t, ace)
				assert.Equal(t, uint8(windows.ACCESS_ALLOWED_ACE_TYPE), ace.Header.AceType)
				assert.Zero(t, ace.Header.AceFlags&windows.INHERITED_ACE)
				assert.True(t, aceGrantsFullControl(ace.Mask))
				if test.directory {
					assert.Equal(
						t,
						uint8(windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE),
						ace.Header.AceFlags&uint8(windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE),
					)
				}
				aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
				switch {
				case userSID.Equals(aceSID):
					seen["user"] = true
				case systemSID.Equals(aceSID):
					seen["system"] = true
				default:
					t.Fatalf("unexpected SID in private DACL: %s", aceSID.String())
				}
			}
			assert.True(t, seen["user"])
			assert.True(t, seen["system"] || userSID.Equals(systemSID))
			runtime.KeepAlive(descriptor)
		})
	}
}
