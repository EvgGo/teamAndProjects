package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func CreateProjectTeamModeToModel(
	mode workspacev1.CreateProjectTeamMode,
) (models.CreateProjectTeamMode, bool) {
	switch mode {
	case workspacev1.CreateProjectTeamMode_CREATE_PROJECT_TEAM_MODE_UNSPECIFIED:
		return models.CreateProjectTeamModeUnspecified, true

	case workspacev1.CreateProjectTeamMode_CREATE_PROJECT_TEAM_MODE_AUTO_GENERATE:
		return models.CreateProjectTeamModeAutoGenerate, true

	case workspacev1.CreateProjectTeamMode_CREATE_PROJECT_TEAM_MODE_ATTACH_EXISTING_BY_NAME:
		return models.CreateProjectTeamModeAttachExistingByName, true

	case workspacev1.CreateProjectTeamMode_CREATE_PROJECT_TEAM_MODE_CREATE_NEW_WITH_NAME:
		return models.CreateProjectTeamModeCreateNewWithName, true

	default:
		return "", false
	}
}
