package grpc

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"strconv"
	"teamAndProjects/internal/models"
)

// projectPublicToProto конвертирует публичный проект модели в proto
func projectPublicToProto(p *models.ProjectPublic) *workspacev1.ProjectPublic {

	if p == nil {
		return nil
	}

	out := &workspacev1.ProjectPublic{
		Id:          p.ID,
		TeamId:      p.TeamID,
		Name:        p.Name,
		Description: p.Description,
		Status:      projectStatusFromModel(p.Status),
		IsOpen:      p.IsOpen,
		StartedAt:   dateFromTime(p.StartedAt),
		CreatedAt:   dateFromTime(p.CreatedAt),
		SkillIds:    intSkillIDsToProto(p.SkillIDs),
		Skills:      projectSkillsToProto(p.Skills),
	}

	if p.FinishedAt != nil {
		out.FinishedAt = dateFromTime(*p.FinishedAt)
	}

	return out
}

func intSkillIDsToProto(ids []int) []string {

	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strconv.FormatInt(int64(id), 10))
	}
	return out
}

func projectSkillsToProto(skills []models.ProjectSkill) []*workspacev1.ProjectSkill {

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

func publicProjectRowToProto(row *models.PublicProjectRow) *workspacev1.ProjectPublic {

	if row == nil {
		return nil
	}

	out := projectPublicToProto(&row.Project)
	if out == nil {
		return nil
	}

	if row.ProfileSkillMatchPercent != nil {
		out.ProfileSkillMatchPercent = row.ProfileSkillMatchPercent
	}

	return out
}
