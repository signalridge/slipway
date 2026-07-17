package autopilot

import "github.com/signalridge/slipway/internal/runstore"

// DurabilityCapability describes the crash-durability boundary available to
// the Run journal on the current platform.
type DurabilityCapability struct {
	Level         string
	FileSync      bool
	DirectorySync bool
	Limitation    string
}

// PlatformDurability reports the Run journal durability available on this OS.
func PlatformDurability() DurabilityCapability {
	capability := runstore.PlatformDurability()
	return DurabilityCapability{
		Level:         capability.Level,
		FileSync:      capability.FileSync,
		DirectorySync: capability.DirectorySync,
		Limitation:    capability.Limitation,
	}
}
