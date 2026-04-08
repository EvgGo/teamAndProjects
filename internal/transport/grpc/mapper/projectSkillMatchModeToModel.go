package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func ProjectSkillMatchModeToModel(
	in workspacev1.ProjectSkillMatchMode,
	hasSkillFilter bool,
) models.ProjectSkillMatchMode {
	switch in {
	case workspacev1.ProjectSkillMatchMode_PROJECT_SKILL_MATCH_MODE_ANY:
		return models.ProjectSkillMatchModeAny
	case workspacev1.ProjectSkillMatchMode_PROJECT_SKILL_MATCH_MODE_ALL:
		return models.ProjectSkillMatchModeAll
	default:
		if hasSkillFilter {
			// по умолчанию для фильтра по skills лучше ALL
			return models.ProjectSkillMatchModeAll
		}
		return models.ProjectSkillMatchModeUnspecified
	}
}
