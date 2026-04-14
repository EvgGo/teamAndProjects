package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func ProjectMemberToProto(member *models.ProjectMember) *workspacev1.ProjectMember {
	if member == nil {
		return nil
	}

	return &workspacev1.ProjectMember{
		ProjectId: member.ProjectID,
		UserId:    member.UserID,
		Rights:    ProjectRightsToProto(member.Rights),
	}
}
