package model

// UserPhase is the user-facing lifecycle phase, mapped from internal WorkflowState.
type UserPhase string

const (
	PhaseIntake    UserPhase = "Intake"
	PhasePlanning  UserPhase = "Planning"
	PhaseBuilding  UserPhase = "Building"
	PhaseReviewing UserPhase = "Reviewing"
	PhaseDone      UserPhase = "Done"
)

func (p UserPhase) String() string { return string(p) }

func (p UserPhase) IsValid() bool {
	switch p {
	case PhaseIntake, PhasePlanning, PhaseBuilding, PhaseReviewing, PhaseDone:
		return true
	default:
		return false
	}
}

// PhaseFor maps an internal WorkflowState to the user-visible UserPhase.
func PhaseFor(s WorkflowState) UserPhase {
	switch s {
	case StateS0Intake:
		return PhaseIntake
	case StateS1Plan:
		return PhasePlanning
	case StateS2Implement:
		return PhaseBuilding
	case StateS3Review:
		return PhaseReviewing
	case StateDone:
		return PhaseDone
	default:
		return PhaseIntake
	}
}
