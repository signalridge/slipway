package model

type TaskKind string

const (
	TaskKindCode          TaskKind = "code"
	TaskKindTest          TaskKind = "test"
	TaskKindDoc           TaskKind = "doc"
	TaskKindOps           TaskKind = "ops"
	TaskKindVerification  TaskKind = "verification"
	TaskKindInvestigation TaskKind = "investigation"
	TaskKindOther         TaskKind = "other"
)

func (k TaskKind) String() string { return string(k) }

func (k TaskKind) IsValid() bool {
	switch k {
	case TaskKindCode, TaskKindTest, TaskKindDoc, TaskKindOps, TaskKindVerification, TaskKindInvestigation, TaskKindOther:
		return true
	default:
		return false
	}
}

// ShouldSkipOnResume returns true if a completed task of this kind can be
// safely skipped during checkpoint resume. Code, doc, ops, other, and empty
// kinds are skip-safe; test, verification, and investigation kinds are not.
func (k TaskKind) ShouldSkipOnResume() bool {
	switch k {
	case TaskKindTest, TaskKindVerification, TaskKindInvestigation:
		return false
	default:
		// code, doc, ops, other, empty — safe to skip.
		return true
	}
}
