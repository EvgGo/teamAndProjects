package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func ListTeamMemberDetailsResponseToProto(
	result *models.ListTeamMemberDetailsResult,
) *workspacev1.ListTeamMemberDetailsResponse {
	members := make([]*workspacev1.TeamMemberDetails, 0, len(result.Members))

	for _, member := range result.Members {
		members = append(members, TeamMemberDetailsToProto(member))
	}

	return &workspacev1.ListTeamMemberDetailsResponse{
		Members:       members,
		NextPageToken: result.NextPageToken,
		MyRights:      TeamRightsToProto(result.MyRights),
		Capabilities:  TeamCapabilitiesToProto(result.Capabilities),
	}
}
