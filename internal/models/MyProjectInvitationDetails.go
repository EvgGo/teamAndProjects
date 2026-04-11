package models

import "time"

type UserPublicSummary struct {
	UserID    string
	FirstName string
	LastName  string
}

type ProjectInvitationProjectSummary struct {
	ID          string
	Name        string
	Description string

	Status ProjectStatus
	IsOpen bool

	StartedAt  time.Time
	FinishedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time

	Skills []ProjectSkill
}

type MyProjectInvitationDetails struct {
	Invitation    ProjectInvitation
	Project       ProjectInvitationProjectSummary
	InvitedByUser UserPublicSummary
	SkillMatch    SkillMatchSummary
}
