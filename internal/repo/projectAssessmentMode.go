package repo

import (
	"fmt"
	"strings"
	"teamAndProjects/internal/models"
)

func assessmentModeFromAssessmentText(mode string) (models.ProjectAssessmentMode, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "subtopic":
		return models.ProjectAssessmentModeSubtopic, nil
	case "global":
		return models.ProjectAssessmentModeGlobal, nil
	default:
		return models.ProjectAssessmentModeUnspecified, fmt.Errorf("unknown assessment mode: %q", mode)
	}
}

func requirementModeFromDBSmallint(mode int16) (models.ProjectAssessmentMode, error) {
	switch mode {
	case 1:
		return models.ProjectAssessmentModeSubtopic, nil
	case 2:
		return models.ProjectAssessmentModeGlobal, nil
	default:
		return models.ProjectAssessmentModeUnspecified, fmt.Errorf("unknown requirement mode: %d", mode)
	}
}

func requirementModeToDBSmallint(mode models.ProjectAssessmentMode) (int16, error) {
	switch mode {
	case models.ProjectAssessmentModeSubtopic:
		return 1, nil
	case models.ProjectAssessmentModeGlobal:
		return 2, nil
	default:
		return 0, fmt.Errorf("unknown project assessment mode: %d", mode)
	}
}
