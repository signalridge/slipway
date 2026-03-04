package model

type TaskKind string

const (
	TaskKindCode  TaskKind = "code"
	TaskKindTest  TaskKind = "test"
	TaskKindDoc   TaskKind = "doc"
	TaskKindOps   TaskKind = "ops"
	TaskKindOther TaskKind = "other"
)

func (k TaskKind) IsValid() bool {
	switch k {
	case TaskKindCode, TaskKindTest, TaskKindDoc, TaskKindOps, TaskKindOther:
		return true
	default:
		return false
	}
}
