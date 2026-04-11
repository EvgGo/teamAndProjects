package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func SkillMatchSummaryToProto(m models.SkillMatchSummary) *workspacev1.SkillMatchSummary {
	out := &workspacev1.SkillMatchSummary{
		MatchPercent:            m.MatchPercent,
		MatchedSkillsCount:      m.MatchedSkillsCount,
		TotalProjectSkillsCount: m.TotalProjectSkillsCount,
	}

	for _, skill := range m.MatchedSkills {
		out.MatchedSkills = append(out.MatchedSkills, &workspacev1.ProjectSkill{
			Id:   skill.ID,
			Name: skill.Name,
		})
	}

	for _, skill := range m.MissingProjectSkills {
		out.MissingProjectSkills = append(out.MissingProjectSkills, &workspacev1.ProjectSkill{
			Id:   skill.ID,
			Name: skill.Name,
		})
	}

	return out
}
