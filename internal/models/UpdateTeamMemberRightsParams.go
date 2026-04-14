package models

type UpdateTeamMemberRightsParams struct {
	TeamID string
	UserID string

	RootRights               *bool
	ManagerTeam              *bool
	ManagerMembers           *bool
	ManagerMemberDuties      *bool
	ManagerProjectAssignment *bool
	ManagerProjectRights     *bool
	ManagerProjects          *bool
}
