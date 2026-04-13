package models

type ProjectMemberUserSummary struct {
	UserID    string
	FirstName string
	LastName  string
	About     string
	Skills    []ProjectSkill
}

type ProjectMemberCapabilities struct {
	CanEditRights        bool
	CanRemoveFromProject bool
	CanRemoveFromTeam    bool
}

type ProjectMemberDetails struct {
	ProjectID string
	UserID    string

	Rights ProjectRights
	User   ProjectMemberUserSummary

	IsTeamMember     bool
	TeamDuties       *string
	IsProjectCreator bool
	IsTeamFounder    bool
	IsTeamLead       bool
	IsMe             bool

	Capabilities ProjectMemberCapabilities
}

type ProjectMemberDetailsRow struct {
	ProjectID string
	UserID    string

	Rights ProjectRights

	IsTeamMember     bool
	TeamDuties       *string
	IsProjectCreator bool
	IsTeamFounder    bool
	IsTeamLead       bool
}

type ListProjectMemberDetailsFilter struct {
	ProjectID string
	PageSize  int32
	PageToken string
}

type ListProjectMemberDetailsResult struct {
	Members              []ProjectMemberDetails
	NextPageToken        string
	MyRights             ProjectRights
	CanManageTeamMembers bool
}
