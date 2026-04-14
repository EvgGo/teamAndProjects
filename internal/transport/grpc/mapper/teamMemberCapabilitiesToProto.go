package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func TeamMemberCapabilitiesToProto(
	capabilities models.TeamMemberCapabilities,
) *workspacev1.TeamMemberCapabilities {
	return &workspacev1.TeamMemberCapabilities{
		CanUpdateDuties:    capabilities.CanUpdateDuties,
		CanUpdateRights:    capabilities.CanUpdateRights,
		CanRemoveFromTeam:  capabilities.CanRemoveFromTeam,
		CanAssignToProject: capabilities.CanAssignToProject,
	}
}
