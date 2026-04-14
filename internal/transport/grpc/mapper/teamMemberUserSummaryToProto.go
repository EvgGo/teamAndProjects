package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func TeamMemberUserSummaryToProto(
	user models.TeamMemberUserSummary,
) *workspacev1.TeamMemberUserSummary {
	skills := make([]*workspacev1.ProjectSkill, 0, len(user.Skills))

	for _, skill := range user.Skills {
		skills = append(skills, &workspacev1.ProjectSkill{
			Id:   skill.ID,
			Name: skill.Name,
		})
	}

	return &workspacev1.TeamMemberUserSummary{
		UserId:    user.UserID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		About:     user.About,
		Skills:    skills,
	}
}
