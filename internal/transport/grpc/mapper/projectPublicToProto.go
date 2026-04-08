package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"strconv"
	"teamAndProjects/internal/models"
)

// ProjectPublicToProto конвертирует публичный проект модели в proto
func ProjectPublicToProto(p *models.ProjectPublic) *workspacev1.ProjectPublic {

	if p == nil {
		return nil
	}

	out := &workspacev1.ProjectPublic{
		Id:          p.ID,
		TeamId:      p.TeamID,
		Name:        p.Name,
		Description: p.Description,
		Status:      ProjectStatusFromModel(p.Status),
		IsOpen:      p.IsOpen,
		StartedAt:   DateFromTime(p.StartedAt),
		CreatedAt:   DateFromTime(p.CreatedAt),
		SkillIds:    IntSkillIDsToProto(p.SkillIDs),
		Skills:      ProjectSkillsToProto(p.Skills),
	}

	if p.FinishedAt != nil {
		out.FinishedAt = DateFromTime(*p.FinishedAt)
	}

	return out
}

func IntSkillIDsToProto(ids []int) []string {

	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strconv.FormatInt(int64(id), 10))
	}
	return out
}

func ProjectSkillsToProto(skills []models.ProjectSkill) []*workspacev1.ProjectSkill {

	if len(skills) == 0 {
		return nil
	}

	out := make([]*workspacev1.ProjectSkill, 0, len(skills))
	for _, s := range skills {
		out = append(out, &workspacev1.ProjectSkill{
			Id:   strconv.Itoa(s.ID),
			Name: s.Name,
		})
	}
	return out
}

func PublicProjectRowToProto(row *models.PublicProjectRow) *workspacev1.ProjectPublic {

	if row == nil {
		return nil
	}

	out := ProjectPublicToProto(&row.Project)
	if out == nil {
		return nil
	}

	if row.ProfileSkillMatchPercent != nil {
		out.ProfileSkillMatchPercent = row.ProfileSkillMatchPercent
	}

	return out
}
