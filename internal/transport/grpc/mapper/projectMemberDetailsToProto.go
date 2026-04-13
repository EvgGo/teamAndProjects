package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func ProjectMemberDetailsToProto(member models.ProjectMemberDetails) *workspacev1.ProjectMemberDetails {
	return &workspacev1.ProjectMemberDetails{
		ProjectId:        member.ProjectID,
		UserId:           member.UserID,
		Rights:           ProjectRightsToProto(member.Rights),
		User:             ProjectMemberUserSummaryToProto(member.User),
		IsTeamMember:     member.IsTeamMember,
		TeamDuties:       member.TeamDuties,
		IsProjectCreator: member.IsProjectCreator,
		IsTeamFounder:    member.IsTeamFounder,
		IsTeamLead:       member.IsTeamLead,
		IsMe:             member.IsMe,
		Capabilities:     ProjectMemberCapabilitiesToProto(member.Capabilities),
	}
}

func ProjectMemberUserSummaryToProto(summary models.ProjectMemberUserSummary) *workspacev1.ProjectMemberUserSummary {
	return &workspacev1.ProjectMemberUserSummary{
		UserId:    summary.UserID,
		FirstName: summary.FirstName,
		LastName:  summary.LastName,
		About:     summary.About,
		Skills:    ProjectSkillsToProto(summary.Skills),
	}
}

func ProjectMemberCapabilitiesToProto(caps models.ProjectMemberCapabilities) *workspacev1.ProjectMemberCapabilities {
	return &workspacev1.ProjectMemberCapabilities{
		CanEditRights:        caps.CanEditRights,
		CanRemoveFromProject: caps.CanRemoveFromProject,
		CanRemoveFromTeam:    caps.CanRemoveFromTeam,
	}
}
