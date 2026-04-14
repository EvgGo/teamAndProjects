package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func TeamMemberDetailsToProto(
	member models.TeamMemberDetails,
) *workspacev1.TeamMemberDetails {

	projects := make([]*workspacev1.TeamMemberProjectSummary, 0, len(member.Projects))
	for _, project := range member.Projects {
		projects = append(projects, &workspacev1.TeamMemberProjectSummary{
			ProjectId:     project.ProjectID,
			ProjectName:   project.ProjectName,
			ProjectStatus: ProjectStatusToProto(project.ProjectStatus),
		})
	}

	return &workspacev1.TeamMemberDetails{
		TeamId:       member.TeamID,
		UserId:       member.UserID,
		Duties:       member.Duties,
		JoinedAt:     DateFromTime(member.JoinedAt),
		Rights:       TeamRightsToProto(member.Rights),
		User:         TeamMemberUserSummaryToProto(member.User),
		IsMe:         member.IsMe,
		IsFounder:    member.IsFounder,
		IsLead:       member.IsLead,
		Projects:     projects,
		Capabilities: TeamMemberCapabilitiesToProto(member.Capabilities),
	}
}
