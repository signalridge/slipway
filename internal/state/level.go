package state

import (
	"fmt"
	"time"

	"github.com/signalridge/speclane/internal/model"
)

// ChangeManifest captures persisted governed manifest metadata.
type ChangeManifest struct {
	RequestID      string      `yaml:"request_id,omitempty" json:"request_id,omitempty"`
	Slug           string      `yaml:"slug,omitempty" json:"slug,omitempty"`
	CreatedAtLevel model.Level `yaml:"created_at_level" json:"created_at_level"`
}

// ApplyLevelPivot updates top-level runtime level metadata only.
func ApplyLevelPivot(
	change *model.ChangeState,
	level model.Level,
	levelSource model.LevelSource,
	reason string,
	at time.Time,
	maxLevelHistoryEntries int,
) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}
	if !level.IsValid() {
		return fmt.Errorf("invalid level %q", level)
	}
	if !levelSource.IsValid() {
		return fmt.Errorf("invalid level_source %q", levelSource)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}

	change.Level = level
	change.LevelSource = levelSource
	change.LastLevelUpdateAt = &at
	change.LevelHistory = append(change.LevelHistory, model.LevelHistoryEvent{
		Level:       level,
		LevelSource: levelSource,
		Reason:      reason,
		At:          at,
	})
	change.LevelHistory = model.TruncateLevelHistory(change.LevelHistory, maxLevelHistoryEntries)
	return nil
}
