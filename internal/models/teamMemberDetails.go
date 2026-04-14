package models

import "time"

type TeamRights struct {
	RootRights               bool
	ManagerTeam              bool
	ManagerMembers           bool
	ManagerMemberDuties      bool
	ManagerProjectAssignment bool
	ManagerProjectRights     bool
	ManagerProjects          bool
}

type TeamCapabilities struct {
	CanUpdateTeam              bool
	CanDeleteTeam              bool
	CanManageMembers           bool
	CanUpdateMemberDuties      bool
	CanUpdateMemberRights      bool
	CanAssignMembersToProjects bool
	CanManageProjects          bool
}

type TeamMemberCapabilities struct {
	CanUpdateDuties    bool
	CanUpdateRights    bool
	CanRemoveFromTeam  bool
	CanAssignToProject bool
}

type SkillSummary struct {
	ID   string
	Name string
}

type TeamMemberUserSummary struct {
	UserID    string
	FirstName string
	LastName  string
	About     string
	Skills    []SkillSummary
}

type TeamMemberProjectSummary struct {
	ProjectID     string
	ProjectName   string
	ProjectStatus string
}

type TeamAccessRow struct {
	TeamID    string
	FounderID string
	LeadID    string
	MyRights  TeamRights
}

type TeamMemberDetailsRow struct {
	TeamID   string
	UserID   string
	Duties   string
	JoinedAt time.Time

	Rights TeamRights

	IsFounder bool
	IsLead    bool
}

type TeamMemberProjectSummaryRow struct {
	UserID        string
	ProjectID     string
	ProjectName   string
	ProjectStatus string
}

type ListTeamMemberDetailsParams struct {
	TeamID    string
	Query     string
	SkillIDs  []string
	PageSize  int32
	PageToken string
}

type TeamMemberDetails struct {
	TeamID   string
	UserID   string
	Duties   string
	JoinedAt time.Time

	Rights TeamRights
	User   TeamMemberUserSummary

	IsMe      bool
	IsFounder bool
	IsLead    bool

	Projects     []TeamMemberProjectSummary
	Capabilities TeamMemberCapabilities
}

type ListTeamMemberDetailsResult struct {
	Members       []TeamMemberDetails
	NextPageToken string
	MyRights      TeamRights
	Capabilities  TeamCapabilities
}
