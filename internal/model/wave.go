package model

type TaskKind string

const (
	TaskKindImplementation TaskKind = "implementation"
	TaskKindReview         TaskKind = "review"
	TaskKindVerification   TaskKind = "verification"
	TaskKindOther          TaskKind = "other"
)

func (k TaskKind) IsValid() bool {
	switch k {
	case TaskKindImplementation, TaskKindReview, TaskKindVerification, TaskKindOther:
		return true
	default:
		return false
	}
}
