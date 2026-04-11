package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func CandidatePublicSummaryToProto(m models.CandidatePublicSummary) *workspacev1.CandidatePublicSummary {
	out := &workspacev1.CandidatePublicSummary{
		UserId:    m.UserID,
		FirstName: m.FirstName,
		LastName:  m.LastName,
	}

	if m.About != nil {
		out.About = *m.About
	}

	for _, skill := range m.Skills {
		out.Skills = append(out.Skills, &workspacev1.ProjectSkill{
			Id:   skill.ID,
			Name: skill.Name,
		})
	}

	return out
}
