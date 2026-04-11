package models

import "time"

type ProjectInvitationStatus int16

const (
	ProjectInvitationStatusUnspecified ProjectInvitationStatus = 0
	ProjectInvitationStatusPending     ProjectInvitationStatus = 1
	ProjectInvitationStatusAccepted    ProjectInvitationStatus = 2
	ProjectInvitationStatusRejected    ProjectInvitationStatus = 3
	ProjectInvitationStatusRevoked     ProjectInvitationStatus = 4
)

type ProjectInvitation struct {
	ID             string
	ProjectID      string
	InvitedUserID  string
	InvitedBy      string
	Message        string
	Status         ProjectInvitationStatus
	DecidedBy      *string
	DecidedAt      *time.Time
	CreatedAt      time.Time
	DecisionReason *string
}

type ProjectInvitationDetails struct {
	ID             string
	ProjectID      string
	InvitedUserID  string
	InvitedBy      string
	Message        string
	Status         ProjectInvitationStatus
	DecidedBy      *string
	DecidedAt      *time.Time
	CreatedAt      time.Time
	DecisionReason *string

	Candidate  CandidatePublicSummary
	SkillMatch SkillMatchSummary
}

type MyProjectInvitationItem struct {
	ProjectID     string
	ProjectName   string
	ProjectStatus ProjectStatus
	ProjectIsOpen bool
	Invitation    ProjectInvitation
}

type InvitableProjectItem struct {
	ProjectID     string
	ProjectName   string
	ProjectStatus ProjectStatus
	IsOpen        bool
	MyRights      ProjectRights
}

type CreateProjectInvitationInput struct {
	ID            string
	ProjectID     string
	InvitedUserID string
	InvitedBy     string
	Message       string
}

type DecideProjectInvitationInput struct {
	InvitationID   string
	DecidedBy      string
	DecisionReason *string
	DecidedAt      time.Time
}

type ListProjectInvitationsFilter struct {
	ProjectID string
	Status    ProjectInvitationStatus
	PageSize  int32
	PageToken string
}

type ListProjectInvitationDetailsFilter struct {
	ProjectID string
	Status    ProjectInvitationStatus
	PageSize  int32
	PageToken string
}

type ListMyProjectInvitationsFilter struct {
	UserID    string
	Status    ProjectInvitationStatus
	PageSize  int32
	PageToken string
}

type ListMyInvitableProjectsFilter struct {
	UserID    string
	Query     string
	OnlyOpen  bool
	PageSize  int32
	PageToken string
}
