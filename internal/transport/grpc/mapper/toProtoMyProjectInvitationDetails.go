package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"strconv"
	"teamAndProjects/internal/models"
)

func MyProjectInvitationDetailsToProto(item *models.MyProjectInvitationDetails) *workspacev1.MyProjectInvitationDetails {
	if item == nil {
		return nil
	}

	return &workspacev1.MyProjectInvitationDetails{
		Invitation:    ProjectInvitationToProto(item.Invitation),
		Project:       ProjectInvitationProjectSummaryToProto(item.Project),
		InvitedByUser: UserPublicSummaryToProto(item.InvitedByUser),
		SkillMatch:    SkillMatchSummaryToProto(item.SkillMatch),
	}
}

func UserPublicSummaryToProto(user models.UserPublicSummary) *workspacev1.UserPublicSummary {
	return &workspacev1.UserPublicSummary{
		UserId:    user.UserID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}
}

func ProjectInvitationProjectSummaryToProto(project models.ProjectInvitationProjectSummary) *workspacev1.ProjectInvitationProjectSummary {

	skills := make([]*workspacev1.ProjectSkill, 0, len(project.Skills))

	for _, skill := range project.Skills {
		skills = append(skills, &workspacev1.ProjectSkill{
			Id:   strconv.Itoa(skill.ID),
			Name: skill.Name,
		})
	}

	return &workspacev1.ProjectInvitationProjectSummary{
		Id:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		Status:      ProjectStatusFromModel(project.Status),
		IsOpen:      project.IsOpen,
		StartedAt:   DateFromTime(project.StartedAt),
		FinishedAt:  DateFromTimePtr(project.FinishedAt),
		CreatedAt:   DateFromTime(project.CreatedAt),
		UpdatedAt:   DateFromTime(project.UpdatedAt),
		Skills:      skills,
	}
}
