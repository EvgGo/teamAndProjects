package projectsvc

import "teamAndProjects/internal/models"

func uniqueProjectSkillsByID(skills []models.Skill) []models.Skill {
	if len(skills) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(skills))
	out := make([]models.Skill, 0, len(skills))

	for _, skill := range skills {
		if skill.ID == "" {
			continue
		}
		if _, ok := seen[skill.ID]; ok {
			continue
		}
		seen[skill.ID] = struct{}{}
		out = append(out, skill)
	}

	return out
}
