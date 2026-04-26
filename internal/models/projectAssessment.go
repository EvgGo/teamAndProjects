package models

type ProjectAssessmentMode int32

const (
	ProjectAssessmentModeUnspecified ProjectAssessmentMode = 0
	ProjectAssessmentModeSubtopic    ProjectAssessmentMode = 1
	ProjectAssessmentModeGlobal      ProjectAssessmentMode = 2
)

type ProjectAssessmentRequirement struct {
	AssessmentID    int64
	AssessmentCode  string
	AssessmentTitle string

	SubjectID    int64
	SubjectCode  string
	SubjectTitle string

	Mode     ProjectAssessmentMode
	MinLevel int32
}

type ProjectAssessmentRequirementInput struct {
	AssessmentID int64
	MinLevel     int32
}

type ProjectAssessmentRequirementCheck struct {
	Requirement       ProjectAssessmentRequirement
	HasSavedResult    bool
	CurrentLevel      int32
	MeetsRequirement  bool
	NeedsPassTest     bool
	NeedsImproveLevel bool
}

type ProjectJoinEligibility struct {
	ProjectID string

	CanRequestJoin        bool
	IsProjectOpen         bool
	AlreadyMember         bool
	HasPendingJoinRequest bool
	HasPendingInvitation  bool

	TotalRequirementsCount   int32
	MatchedRequirementsCount int32

	Checks []ProjectAssessmentRequirementCheck
}

type SavedAssessmentResult struct {
	AssessmentID int64
	Level        int32
	Mode         ProjectAssessmentMode
}
