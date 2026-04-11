package models

import "time"

type ListProjectJoinRequestDetailsFilter struct {
	ViewerID  string
	ProjectID string

	Status    *JoinRequestStatus
	PageSize  int32
	PageToken string
}

type CandidatePublicSummary struct {
	UserID            string
	FirstName         string
	LastName          string
	About             *string
	AvatarURL         *string
	IsOpenSuggestions bool
	Skills            []Skill
}

type Skill struct {
	ID   string
	Name string
}

type SkillMatchSummary struct {
	MatchPercent            int32
	MatchedSkillsCount      int32
	TotalProjectSkillsCount int32
	MatchedSkills           []Skill
	MissingProjectSkills    []Skill
}

type ProjectJoinRequestDetails struct {
	ID          string
	ProjectID   string
	RequesterID string

	Message string
	Status  JoinRequestStatus

	DecidedBy *string
	DecidedAt *time.Time
	CreatedAt time.Time

	Candidate       CandidatePublicSummary
	SkillMatch      SkillMatchSummary
	RejectionReason *string
}
