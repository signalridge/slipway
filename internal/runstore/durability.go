package runstore

import (
	"os"
	"runtime"
)

const (
	DurabilityLevelFileAndDirectory = "file_and_directory_fsync"
	DurabilityLevelFileOnly         = "file_fsync_only"
	DurabilityLimitDirectorySync    = "directory_fsync_unsupported"
)

// DurabilityCapability is stable and machine-readable so doctor and other
// product surfaces can report the platform's actual crash-durability boundary.
type DurabilityCapability struct {
	Level         string `json:"level"`
	FileSync      bool   `json:"file_sync"`
	DirectorySync bool   `json:"directory_sync"`
	Limitation    string `json:"limitation,omitempty"`
}

// PlatformDurability reports the guarantees runstore can request on this OS.
func PlatformDurability() DurabilityCapability {
	if runtime.GOOS == "windows" {
		return DurabilityCapability{
			Level:         DurabilityLevelFileOnly,
			FileSync:      true,
			DirectorySync: false,
			Limitation:    DurabilityLimitDirectorySync,
		}
	}
	return DurabilityCapability{
		Level:         DurabilityLevelFileAndDirectory,
		FileSync:      true,
		DirectorySync: true,
	}
}

func syncDirectoryFile(file *os.File) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return file.Sync()
}
