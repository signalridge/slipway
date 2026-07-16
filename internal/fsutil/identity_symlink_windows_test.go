//go:build windows

package fsutil

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestOpenSymlinkIdentityPinsSymlinkObject(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(directory, "first.txt"), []byte("first"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "second.txt"), []byte("second"), 0o600))
	root, err := os.OpenRoot(directory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	if err := root.Symlink("first.txt", "link"); errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		t.Skip("creating symbolic links requires an unavailable Windows privilege")
	} else {
		require.NoError(t, err)
	}

	expected, err := root.Lstat("link")
	require.NoError(t, err)
	identity, err := openSymlinkIdentity(root, "link")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, identity.Close()) })
	opened, err := identity.Stat()
	require.NoError(t, err)
	require.ErrorIs(t, validateSymlinkTransactionIdentity(opened), ErrFileTransactionSymlinkUnsupported)
	assert.NotZero(t, opened.Mode()&os.ModeSymlink)
	assert.True(t, os.SameFile(expected, opened))
	heldTarget, err := readSymlinkIdentity(root, "link", identity)
	require.NoError(t, err)
	assert.Equal(t, "first.txt", heldTarget)

	require.NoError(t, root.Remove("link"))
	require.NoError(t, root.Symlink("second.txt", "link"))
	recreated, err := root.Lstat("link")
	require.NoError(t, err)
	assert.NotZero(t, recreated.Mode()&os.ModeSymlink)
	assert.False(t, os.SameFile(opened, recreated))
	heldTarget, err = readSymlinkIdentity(root, "link", identity)
	require.NoError(t, err)
	assert.Equal(t, "first.txt", heldTarget)
	target, err := root.Readlink("link")
	require.NoError(t, err)
	assert.Equal(t, "second.txt", target)
}

func TestWindowsSymlinkTransactionsFailClosedWithoutCreationPrivilege(t *testing.T) {
	// This policy assertion deliberately needs no symbolic-link fixture or
	// elevated privilege. It is the non-skippable proof that every Windows link
	// kind is rejected before a transaction may mutate it.
	require.ErrorIs(t, validateSymlinkTransactionIdentity(nil), ErrFileTransactionSymlinkUnsupported)
}

func TestWindowsExistingSymlinkRemainsUntouchedWhenTransactionFailsClosed(t *testing.T) {
	for _, test := range []struct {
		name      string
		targetDir bool
	}{
		{name: "file"},
		{name: "directory", targetDir: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			target := filepath.Join(directory, "target")
			if test.targetDir {
				require.NoError(t, os.Mkdir(target, 0o700))
			} else {
				require.NoError(t, os.WriteFile(target, []byte("target"), 0o600))
			}
			root, err := os.OpenRoot(directory)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, root.Close()) })
			if err := root.Symlink("target", "link"); errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
				t.Skip("native fixture creation requires an unavailable Windows privilege; the policy assertion above is non-skippable")
			} else {
				require.NoError(t, err)
			}

			before, err := root.Lstat("link")
			require.NoError(t, err)
			err = applyFileTransactionForTest([]FileTransactionOp{RemoveFileTransactionOp(filepath.Join(directory, "link"))})
			require.ErrorIs(t, err, ErrFileTransactionSymlinkUnsupported)
			after, err := root.Lstat("link")
			require.NoError(t, err, "unsupported link must remain untouched")
			assert.True(t, os.SameFile(before, after), "fail-closed validation must happen before mutation")
		})
	}
}

func TestWindowsLaterSymlinkFailsBeforeEarlierMutation(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "first.txt")
	target := filepath.Join(directory, "target.txt")
	require.NoError(t, os.WriteFile(first, []byte("before"), 0o600))
	require.NoError(t, os.WriteFile(target, []byte("target"), 0o600))
	root, err := os.OpenRoot(directory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	if err := root.Symlink("target.txt", "link"); errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		t.Skip("native fixture creation requires an unavailable Windows privilege; transaction-wide preflight is covered by the non-skippable policy assertion")
	} else {
		require.NoError(t, err)
	}

	var mutationStarted bool
	err = applyFileTransactionWithHooksForTest([]FileTransactionOp{
		WriteFileTransactionOp(first, []byte("after"), 0o600),
		RemoveFileTransactionOp(filepath.Join(directory, "link")),
	}, fileTransactionHooks{BeforeMutation: func(_, _ string) error {
		mutationStarted = true
		return nil
	}})
	require.ErrorIs(t, err, ErrFileTransactionSymlinkUnsupported)
	assert.False(t, mutationStarted)
	content, readErr := os.ReadFile(first)
	require.NoError(t, readErr)
	assert.Equal(t, "before", string(content))
}

func TestOpenSymlinkIdentityRejectsOtherEntries(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(directory, "regular.txt"), []byte("regular"), 0o600))
	root, err := os.OpenRoot(directory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	identity, err := openSymlinkIdentity(root, "regular.txt")
	require.Error(t, err)
	assert.Nil(t, identity)
	identity, err = openSymlinkIdentity(root, filepath.Join("nested", "link"))
	require.Error(t, err)
	assert.Nil(t, identity)
}

func TestDecodeSymlinkReparseDataUsesSubstituteNameRatherThanDisplayName(t *testing.T) {
	targets := []string{"..\\界🙂.txt", `\root-relative`}
	for _, target := range targets {
		buffer := symlinkReparseDataNamesForTest(target, "display-only.txt", 1)

		actual, err := decodeSymlinkReparseData(buffer, uint32(len(buffer)))
		require.NoError(t, err)
		assert.Equal(t, target, actual)
	}
}

func TestDecodeSymlinkReparseDataNormalizesAbsoluteSubstituteName(t *testing.T) {
	tests := map[string]struct {
		substitute string
		printName  string
		want       string
	}{
		"drive":       {substitute: `\??\C:\real`, printName: `C:\display`, want: `C:\real`},
		"UNC":         {substitute: `\??\UNC\server\share\real`, printName: `\\server\share\display`, want: `\\server\share\real`},
		"volume GUID": {substitute: `\??\Volume{01234567-89ab-cdef-0123-456789abcdef}\real`, printName: `volume display`, want: `\\?\Volume{01234567-89ab-cdef-0123-456789abcdef}\real`},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			buffer := symlinkReparseDataNamesForTest(test.substitute, test.printName, 0)
			actual, err := decodeSymlinkReparseData(buffer, uint32(len(buffer)))
			require.NoError(t, err)
			assert.Equal(t, test.want, actual)
		})
	}
}

func TestDecodeSymlinkReparseDataRejectsUnrestorableOrMalformedData(t *testing.T) {
	tests := map[string]func() []byte{
		"mount point": func() []byte {
			buffer := symlinkReparseDataForTest("target", 0)
			binary.LittleEndian.PutUint32(buffer[0:4], windows.IO_REPARSE_TAG_MOUNT_POINT)
			return buffer
		},
		"unknown flags": func() []byte {
			return symlinkReparseDataForTest("target", 2)
		},
		"declared length beyond result": func() []byte {
			buffer := symlinkReparseDataForTest("target", 0)
			binary.LittleEndian.PutUint16(buffer[4:6], binary.LittleEndian.Uint16(buffer[4:6])+2)
			return buffer
		},
		"substitute name beyond declaration": func() []byte {
			buffer := symlinkReparseDataForTest("target", 0)
			binary.LittleEndian.PutUint16(buffer[10:12], binary.LittleEndian.Uint16(buffer[10:12])+2)
			return buffer
		},
		"empty substitute name": func() []byte {
			return symlinkReparseDataForTest("", 0)
		},
		"relative flag with absolute name": func() []byte {
			return symlinkReparseDataForTest(`C:\target`, 1)
		},
		"relative flag with volume-relative name": func() []byte {
			return symlinkReparseDataForTest(`C:target`, 1)
		},
		"absolute flag with relative name": func() []byte {
			return symlinkReparseDataForTest("target", 0)
		},
		"absolute flag with root-relative name": func() []byte {
			return symlinkReparseDataForTest(`\target`, 0)
		},
		"absolute flag with volume-relative name": func() []byte {
			return symlinkReparseDataForTest(`C:target`, 0)
		},
	}

	for name, makeBuffer := range tests {
		t.Run(name, func(t *testing.T) {
			buffer := makeBuffer()
			_, err := decodeSymlinkReparseData(buffer, uint32(len(buffer)))
			require.Error(t, err)
		})
	}
}

func symlinkReparseDataForTest(target string, flags uint32) []byte {
	return symlinkReparseDataNamesForTest(target, target, flags)
}

func symlinkReparseDataNamesForTest(substitute, printName string, flags uint32) []byte {
	substituteUnits := utf16.Encode([]rune(substitute))
	printUnits := utf16.Encode([]rune(printName))
	const reparseHeaderSize = 8
	const symlinkHeaderSize = 12
	dataLength := symlinkHeaderSize + (len(substituteUnits)+len(printUnits))*2
	buffer := make([]byte, reparseHeaderSize+dataLength)
	binary.LittleEndian.PutUint32(buffer[0:4], windows.IO_REPARSE_TAG_SYMLINK)
	binary.LittleEndian.PutUint16(buffer[4:6], uint16(dataLength))
	binary.LittleEndian.PutUint16(buffer[8:10], 0)
	binary.LittleEndian.PutUint16(buffer[10:12], uint16(len(substituteUnits)*2))
	binary.LittleEndian.PutUint16(buffer[12:14], uint16(len(substituteUnits)*2))
	binary.LittleEndian.PutUint16(buffer[14:16], uint16(len(printUnits)*2))
	binary.LittleEndian.PutUint32(buffer[16:20], flags)
	for index, unit := range append(substituteUnits, printUnits...) {
		binary.LittleEndian.PutUint16(buffer[20+index*2:22+index*2], unit)
	}
	return buffer
}
