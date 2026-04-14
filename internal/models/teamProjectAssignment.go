package models

type AssignTeamMemberToProjectParams struct {
	TeamID        string
	ProjectID     string
	UserID        string
	InitialRights ProjectRights
}

type ListTeamProjectsForAssignmentParams struct {
	TeamID    string
	UserID    string
	Query     string
	PageSize  int32
	PageToken string
}

type TeamProjectAssignmentItem struct {
	ProjectID       string
	ProjectName     string
	ProjectStatus   string
	IsOpen          bool
	IsAlreadyMember bool
	CurrentRights   ProjectRights
}

type ListTeamProjectsForAssignmentResult struct {
	Items         []TeamProjectAssignmentItem
	NextPageToken string
}
