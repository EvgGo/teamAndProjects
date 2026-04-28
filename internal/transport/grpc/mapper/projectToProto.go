package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"strconv"
	"teamAndProjects/internal/models"
)

// ProjectToProto конвертирует модель проекта в protobuf-сообщение
func ProjectToProto(p *models.Project) *workspacev1.Project {

	skillIDs := make([]string, 0, len(p.SkillIDs))
	for _, id := range p.SkillIDs {
		skillIDs = append(skillIDs, strconv.Itoa(id))
	}

	skills := make([]*workspacev1.ProjectSkill, 0, len(p.Skills))
	for _, sk := range p.Skills {
		skills = append(skills, &workspacev1.ProjectSkill{
			Id:   strconv.Itoa(sk.ID),
			Name: sk.Name,
		})
	}

	return &workspacev1.Project{
		Id:                     p.ID,
		TeamId:                 p.TeamID,
		CreatorId:              p.CreatorID,
		Name:                   p.Name,
		Description:            p.Description,
		Status:                 ProjectStatusFromModel(p.Status),
		IsOpen:                 p.IsOpen,
		StartedAt:              DateFromTime(p.StartedAt),
		FinishedAt:             DateFromTimePtr(p.FinishedAt),
		CreatedAt:              DateFromTime(p.CreatedAt),
		UpdatedAt:              DateFromTime(p.UpdatedAt),
		SkillIds:               skillIDs,
		Skills:                 skills,
		MyRights:               ProjectRightsToProto(p.MyRights),
		AssessmentRequirements: ProjectAssessmentRequirementsToProto(p.AssessmentRequirements),
		StagesSummary:          ProjectStagesSummaryToProto(p.StagesSummary),
	}
}
