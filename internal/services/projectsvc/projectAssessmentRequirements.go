package projectsvc

import (
	"context"
	"strings"
	"teamAndProjects/internal/authctx"
	"teamAndProjects/internal/models"
	"teamAndProjects/internal/repo"
)

func (s *Service) SetProjectAssessmentRequirements(
	ctx context.Context,
	projectID string,
	inputs []models.ProjectAssessmentRequirementInput,
) (models.Project, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		return models.Project{}, repo.ErrUnauthenticated
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return models.Project{}, repo.ErrInvalidInput
	}

	project, err := s.Deps.Projects.GetByIDForActor(ctx, projectID, caller)
	if err != nil {
		return models.Project{}, err
	}

	if !project.MyRights.ManagerProjects {
		return models.Project{}, repo.ErrForbidden
	}

	if len(inputs) > 10 {
		return models.Project{}, repo.ErrInvalidInput
	}

	seen := make(map[int64]struct{}, len(inputs))
	assessmentIDs := make([]int64, 0, len(inputs))

	for _, item := range inputs {
		if item.AssessmentID <= 0 {
			return models.Project{}, repo.ErrInvalidInput
		}
		if item.MinLevel < 1 || item.MinLevel > 5 {
			return models.Project{}, repo.ErrInvalidInput
		}
		if _, exists := seen[item.AssessmentID]; exists {
			return models.Project{}, repo.ErrInvalidInput
		}
		seen[item.AssessmentID] = struct{}{}
		assessmentIDs = append(assessmentIDs, item.AssessmentID)
	}

	activeAssessments, err := s.Deps.Assessments.GetActiveByIDs(ctx, assessmentIDs)
	if err != nil {
		return models.Project{}, err
	}

	if len(activeAssessments) != len(assessmentIDs) {
		return models.Project{}, repo.ErrInvalidInput
	}

	requirements := make([]models.ProjectAssessmentRequirement, 0, len(inputs))

	for _, item := range inputs {
		base, ok := activeAssessments[item.AssessmentID]
		if !ok {
			return models.Project{}, repo.ErrInvalidInput
		}

		base.MinLevel = item.MinLevel
		requirements = append(requirements, base)
	}

	if err = s.Deps.ProjectAssessmentRequirements.ReplaceForProject(ctx, projectID, requirements); err != nil {
		return models.Project{}, err
	}

	return s.Deps.Projects.GetByIDForActor(ctx, projectID, caller)
}
