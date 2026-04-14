package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func TeamRightsToProto(rights models.TeamRights) *workspacev1.TeamRights {
	return &workspacev1.TeamRights{
		RootRights:               rights.RootRights,
		ManagerTeam:              rights.ManagerTeam,
		ManagerMembers:           rights.ManagerMembers,
		ManagerMemberDuties:      rights.ManagerMemberDuties,
		ManagerProjectAssignment: rights.ManagerProjectAssignment,
		ManagerProjectRights:     rights.ManagerProjectRights,
		ManagerProjects:          rights.ManagerProjects,
	}
}
