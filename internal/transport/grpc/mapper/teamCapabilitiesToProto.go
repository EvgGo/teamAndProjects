package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func TeamCapabilitiesToProto(
	capabilities models.TeamCapabilities,
) *workspacev1.TeamCapabilities {
	return &workspacev1.TeamCapabilities{
		CanUpdateTeam:              capabilities.CanUpdateTeam,
		CanDeleteTeam:              capabilities.CanDeleteTeam,
		CanManageMembers:           capabilities.CanManageMembers,
		CanUpdateMemberDuties:      capabilities.CanUpdateMemberDuties,
		CanUpdateMemberRights:      capabilities.CanUpdateMemberRights,
		CanAssignMembersToProjects: capabilities.CanAssignMembersToProjects,
		CanManageProjects:          capabilities.CanManageProjects,
	}
}
