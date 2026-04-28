package projectsvc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"teamAndProjects/internal/authctx"
	"teamAndProjects/internal/models"
	"teamAndProjects/internal/repo"
)

// CreateProjectStage создает этап проекта.
// Управлять этапами может участник проекта с manager_tasks или manager_rights
func (s *Service) CreateProjectStage(
	ctx context.Context,
	input models.CreateProjectStageInput,
) (models.ProjectStage, error) {
	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("CreateProjectStage: неаутентифицированный вызов", "projectID", input.ProjectID)
		return models.ProjectStage{}, repo.ErrUnauthenticated
	}

	input.ProjectID = strings.TrimSpace(input.ProjectID)
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	log := s.Deps.Log.With(
		"service_method", "CreateProjectStage",
		"project_id", input.ProjectID,
		"caller", caller,
	)

	log.Info("CreateProjectStage: запрос",
		"title", input.Title,
		"position", input.Position,
		"weight_percent", input.WeightPercent,
		"status", string(input.Status),
		"progress_percent", input.ProgressPercent,
	)

	if input.ProjectID == "" {
		log.Warn("CreateProjectStage: пустой project_id")
		return models.ProjectStage{}, repo.ErrInvalidInput
	}

	if input.Status == "" {
		input.Status = models.ProjectStageStatusPlanned
	}

	if err := validateCreateProjectStageInput(input); err != nil {
		log.Warn("CreateProjectStage: невалидные данные", "err", err)
		return models.ProjectStage{}, err
	}

	if err := s.ensureCanManageProjectStages(ctx, input.ProjectID, caller); err != nil {
		log.Warn("CreateProjectStage: доступ запрещен или ошибка проверки прав", "err", err)
		return models.ProjectStage{}, err
	}

	if input.Position > 0 {
		stages, err := s.Deps.ProjectStages.ListByProjectID(ctx, input.ProjectID)
		if err != nil {
			log.Warn("CreateProjectStage: не удалось получить этапы для проверки position", "err", err)
			return models.ProjectStage{}, err
		}

		maxAllowedPosition := int32(len(stages) + 1)
		if input.Position > maxAllowedPosition {
			log.Warn("CreateProjectStage: position выходит за допустимый диапазон",
				"position", input.Position,
				"max_allowed_position", maxAllowedPosition,
			)
			return models.ProjectStage{}, repo.ErrInvalidProjectStageOrder
		}
	}

	stage, err := s.Deps.ProjectStages.Create(ctx, input)
	if err != nil {
		log.Warn("CreateProjectStage: repo вернул ошибку", "err", err)
		return models.ProjectStage{}, err
	}

	log.Info("CreateProjectStage: этап создан", "stage_id", stage.ID, "position", stage.Position)

	return stage, nil
}

// GetProjectStage возвращает один этап проекта.
// Смотреть этапы может участник проекта или участник команды с root_rights / manager_projects
func (s *Service) GetProjectStage(
	ctx context.Context,
	stageID string,
) (models.ProjectStage, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("GetProjectStage: неаутентифицированный вызов", "stageID", stageID)
		return models.ProjectStage{}, repo.ErrUnauthenticated
	}

	stageID = strings.TrimSpace(stageID)

	log := s.Deps.Log.With(
		"service_method", "GetProjectStage",
		"stage_id", stageID,
		"caller", caller,
	)

	if stageID == "" {
		log.Warn("GetProjectStage: пустой stage_id")
		return models.ProjectStage{}, repo.ErrInvalidInput
	}

	stage, err := s.Deps.ProjectStages.GetByID(ctx, stageID)
	if err != nil {
		log.Warn("GetProjectStage: repo вернул ошибку", "err", err)
		return models.ProjectStage{}, err
	}

	if err = s.ensureCanViewProjectStages(ctx, stage.ProjectID, caller); err != nil {
		log.Warn("GetProjectStage: доступ запрещен или ошибка проверки прав", "project_id", stage.ProjectID, "err", err)
		return models.ProjectStage{}, err
	}

	return stage, nil
}

// ListProjectStages возвращает этапы проекта и summary.
// Summary считается на лету и не хранится в бд
func (s *Service) ListProjectStages(
	ctx context.Context,
	projectID string,
) ([]models.ProjectStage, models.ProjectStagesSummary, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("ListProjectStages: неаутентифицированный вызов", "projectID", projectID)
		return nil, models.ProjectStagesSummary{}, repo.ErrUnauthenticated
	}

	projectID = strings.TrimSpace(projectID)

	log := s.Deps.Log.With(
		"service_method", "ListProjectStages",
		"project_id", projectID,
		"caller", caller,
	)

	if projectID == "" {
		log.Warn("ListProjectStages: пустой project_id")
		return nil, models.ProjectStagesSummary{}, repo.ErrInvalidInput
	}

	if err := s.ensureCanViewProjectStages(ctx, projectID, caller); err != nil {
		log.Warn("ListProjectStages: доступ запрещен или ошибка проверки прав", "err", err)
		return nil, models.ProjectStagesSummary{}, err
	}

	stages, err := s.Deps.ProjectStages.ListByProjectID(ctx, projectID)
	if err != nil {
		log.Warn("ListProjectStages: ошибка получения этапов", "err", err)
		return nil, models.ProjectStagesSummary{}, err
	}

	summary, err := s.Deps.ProjectStages.GetSummary(ctx, projectID)
	if err != nil {
		log.Warn("ListProjectStages: ошибка расчета summary", "err", err)
		return nil, models.ProjectStagesSummary{}, err
	}

	log.Debug("ListProjectStages: этапы получены",
		"stages_count", len(stages),
		"total_weight_percent", summary.TotalWeightPercent,
		"total_progress_percent", summary.TotalProgressPercent,
	)

	return stages, summary, nil
}

// UpdateProjectStage обновляет редактируемые поля этапа
// position и score через этот метод не меняются
func (s *Service) UpdateProjectStage(
	ctx context.Context,
	input models.UpdateProjectStageInput,
) (models.ProjectStage, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("UpdateProjectStage: неаутентифицированный вызов", "stageID", input.StageID)
		return models.ProjectStage{}, repo.ErrUnauthenticated
	}

	input.StageID = strings.TrimSpace(input.StageID)

	log := s.Deps.Log.With(
		"service_method", "UpdateProjectStage",
		"stage_id", input.StageID,
		"caller", caller,
	)

	if input.StageID == "" {
		log.Warn("UpdateProjectStage: пустой stage_id")
		return models.ProjectStage{}, repo.ErrInvalidInput
	}

	stage, err := s.Deps.ProjectStages.GetByID(ctx, input.StageID)
	if err != nil {
		log.Warn("UpdateProjectStage: не удалось получить этап", "err", err)
		return models.ProjectStage{}, err
	}

	if err = s.ensureCanManageProjectStages(ctx, stage.ProjectID, caller); err != nil {
		log.Warn("UpdateProjectStage: доступ запрещен или ошибка проверки прав", "project_id", stage.ProjectID, "err", err)
		return models.ProjectStage{}, err
	}

	normalizeUpdateProjectStageInput(&input)

	if err = validateUpdateProjectStageInput(input); err != nil {
		log.Warn("UpdateProjectStage: невалидные данные", "err", err)
		return models.ProjectStage{}, err
	}

	if isEmptyProjectStageUpdate(input) {
		log.Debug("UpdateProjectStage: нет полей для обновления, возвращаем текущий этап")
		return stage, nil
	}

	updated, err := s.Deps.ProjectStages.Update(ctx, input)
	if err != nil {
		log.Warn("UpdateProjectStage: repo вернул ошибку", "err", err)
		return models.ProjectStage{}, err
	}

	log.Info("UpdateProjectStage: этап обновлен", "project_id", updated.ProjectID)

	return updated, nil
}

// DeleteProjectStage удаляет этап и пересчитывает позиции оставшихся этапов проекта
func (s *Service) DeleteProjectStage(
	ctx context.Context,
	stageID string,
) error {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("DeleteProjectStage: неаутентифицированный вызов", "stageID", stageID)
		return repo.ErrUnauthenticated
	}

	stageID = strings.TrimSpace(stageID)

	log := s.Deps.Log.With(
		"service_method", "DeleteProjectStage",
		"stage_id", stageID,
		"caller", caller,
	)

	if stageID == "" {
		log.Warn("DeleteProjectStage: пустой stage_id")
		return repo.ErrInvalidInput
	}

	err := s.Deps.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		stage, err := s.Deps.ProjectStages.GetByID(txCtx, stageID)
		if err != nil {
			return err
		}

		if err = s.ensureCanManageProjectStages(txCtx, stage.ProjectID, caller); err != nil {
			return err
		}

		deleted, err := s.Deps.ProjectStages.Delete(txCtx, stageID)
		if err != nil {
			return err
		}

		if err = s.Deps.ProjectStages.CompactPositions(txCtx, deleted.ProjectID); err != nil {
			return err
		}

		log.Info("DeleteProjectStage: этап удален",
			"project_id", deleted.ProjectID,
			"deleted_position", deleted.Position,
		)

		return nil
	})
	if err != nil {
		log.Warn("DeleteProjectStage: ошибка транзакции", "err", err)
		return err
	}

	return nil
}

// ReorderProjectStages сохраняет новый порядок этапов проекта
// Frontend обязан передать полный список этапов проекта без пропусков позиций
func (s *Service) ReorderProjectStages(
	ctx context.Context,
	projectID string,
	items []models.ProjectStageOrderItem,
) ([]models.ProjectStage, models.ProjectStagesSummary, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("ReorderProjectStages: неаутентифицированный вызов", "projectID", projectID)
		return nil, models.ProjectStagesSummary{}, repo.ErrUnauthenticated
	}

	projectID = strings.TrimSpace(projectID)

	log := s.Deps.Log.With(
		"service_method", "ReorderProjectStages",
		"project_id", projectID,
		"caller", caller,
	)

	if projectID == "" {
		log.Warn("ReorderProjectStages: пустой project_id")
		return nil, models.ProjectStagesSummary{}, repo.ErrInvalidInput
	}

	var outStages []models.ProjectStage
	var outSummary models.ProjectStagesSummary

	err := s.Deps.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.ensureCanManageProjectStages(txCtx, projectID, caller); err != nil {
			return err
		}

		currentStages, err := s.Deps.ProjectStages.ListByProjectID(txCtx, projectID)
		if err != nil {
			return err
		}

		if err = validateProjectStageOrder(currentStages, items); err != nil {
			return err
		}

		outStages, err = s.Deps.ProjectStages.Reorder(txCtx, projectID, items)
		if err != nil {
			return err
		}

		outSummary, err = s.Deps.ProjectStages.GetSummary(txCtx, projectID)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		log.Warn("ReorderProjectStages: ошибка транзакции", "err", err)
		return nil, models.ProjectStagesSummary{}, err
	}

	log.Info("ReorderProjectStages: порядок этапов обновлен", "stages_count", len(outStages))

	return outStages, outSummary, nil
}

// EvaluateProjectStage выставляет или обновляет оценку качества выполнения этапа
func (s *Service) EvaluateProjectStage(
	ctx context.Context,
	input models.EvaluateProjectStageInput,
) (models.ProjectStage, error) {

	caller := authctx.MustUserID(ctx)
	if caller == "" {
		s.Deps.Log.Warn("EvaluateProjectStage: неаутентифицированный вызов", "stageID", input.StageID)
		return models.ProjectStage{}, repo.ErrUnauthenticated
	}

	input.StageID = strings.TrimSpace(input.StageID)
	input.ScoreComment = strings.TrimSpace(input.ScoreComment)

	log := s.Deps.Log.With(
		"service_method", "EvaluateProjectStage",
		"stage_id", input.StageID,
		"caller", caller,
	)

	if input.StageID == "" {
		log.Warn("EvaluateProjectStage: пустой stage_id")
		return models.ProjectStage{}, repo.ErrInvalidInput
	}

	if input.Score < 1 || input.Score > 5 {
		log.Warn("EvaluateProjectStage: невалидная оценка", "score", input.Score)
		return models.ProjectStage{}, repo.ErrInvalidProjectStageScore
	}

	stage, err := s.Deps.ProjectStages.GetByID(ctx, input.StageID)
	if err != nil {
		log.Warn("EvaluateProjectStage: не удалось получить этап", "err", err)
		return models.ProjectStage{}, err
	}

	if err = s.ensureCanManageProjectStages(ctx, stage.ProjectID, caller); err != nil {
		log.Warn("EvaluateProjectStage: доступ запрещен или ошибка проверки прав", "project_id", stage.ProjectID, "err", err)
		return models.ProjectStage{}, err
	}

	input.EvaluatedBy = caller
	input.EvaluatedAt = s.Deps.Clock()

	updated, err := s.Deps.ProjectStages.Evaluate(ctx, input)
	if err != nil {
		log.Warn("EvaluateProjectStage: repo вернул ошибку", "err", err)
		return models.ProjectStage{}, err
	}

	log.Info("EvaluateProjectStage: этап оценен",
		"project_id", updated.ProjectID,
		"score", input.Score,
	)

	return updated, nil
}

// ensureCanManageProjectStages проверяет право управлять этапами проекта.
// Доступ есть у участника проекта с manager_tasks или manager_rights
// Создателя проекта тоже пропускаем как владельца проекта
func (s *Service) ensureCanManageProjectStages(
	ctx context.Context,
	projectID string,
	actorID string,
) error {
	projectID = strings.TrimSpace(projectID)
	actorID = strings.TrimSpace(actorID)

	if projectID == "" || actorID == "" {
		return repo.ErrInvalidInput
	}

	project, err := s.Deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	if project.CreatorID == actorID {
		return nil
	}

	member, err := s.Deps.ProjectMembers.GetMember(ctx, projectID, actorID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return repo.ErrForbidden
		}
		return fmt.Errorf("get project member: %w", err)
	}

	if !member.Rights.ManagerTasks && !member.Rights.ManagerRights {
		return repo.ErrForbidden
	}

	return nil
}

// ensureCanViewProjectStages проверяет право смотреть этапы проекта.
// Смотреть могут:
// - участники проекта;
// - создатель проекта;
// - участники команды с root_rights;
// - участники команды с manager_projects
func (s *Service) ensureCanViewProjectStages(
	ctx context.Context,
	projectID string,
	actorID string,
) error {

	projectID = strings.TrimSpace(projectID)
	actorID = strings.TrimSpace(actorID)

	if projectID == "" || actorID == "" {
		return repo.ErrInvalidInput
	}

	project, err := s.Deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	if project.CreatorID == actorID {
		return nil
	}

	_, err = s.Deps.ProjectMembers.GetMember(ctx, projectID, actorID)
	if err == nil {
		return nil
	}
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return fmt.Errorf("get project member: %w", err)
	}

	team, err := s.Deps.Teams.GetByIDForActor(ctx, project.TeamID, actorID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) || errors.Is(err, repo.ErrForbidden) {
			return repo.ErrForbidden
		}
		return fmt.Errorf("get team for actor: %w", err)
	}

	if team.MyRights.RootRights || team.MyRights.ManagerProjects {
		return nil
	}

	return repo.ErrForbidden
}

func validateCreateProjectStageInput(input models.CreateProjectStageInput) error {

	if input.Title == "" {
		return repo.ErrInvalidProjectStageTitle
	}

	if input.Position < 0 {
		return repo.ErrInvalidProjectStageOrder
	}

	if input.WeightPercent < 0 || input.WeightPercent > 100 {
		return repo.ErrInvalidProjectStageWeight
	}

	if input.ProgressPercent < 0 || input.ProgressPercent > 100 {
		return repo.ErrInvalidProjectStageProgress
	}

	if !isValidProjectStageStatus(input.Status) {
		return repo.ErrInvalidProjectStageStatus
	}

	return nil
}

func normalizeUpdateProjectStageInput(input *models.UpdateProjectStageInput) {

	if input == nil {
		return
	}

	if input.Title != nil {
		v := strings.TrimSpace(*input.Title)
		input.Title = &v
	}

	if input.Description != nil {
		v := strings.TrimSpace(*input.Description)
		input.Description = &v
	}
}

func validateUpdateProjectStageInput(input models.UpdateProjectStageInput) error {

	if input.Title != nil && strings.TrimSpace(*input.Title) == "" {
		return repo.ErrInvalidProjectStageTitle
	}

	if input.WeightPercent != nil {
		if *input.WeightPercent < 0 || *input.WeightPercent > 100 {
			return repo.ErrInvalidProjectStageWeight
		}
	}

	if input.ProgressPercent != nil {
		if *input.ProgressPercent < 0 || *input.ProgressPercent > 100 {
			return repo.ErrInvalidProjectStageProgress
		}
	}

	if input.Status != nil {
		if !isValidProjectStageStatus(*input.Status) {
			return repo.ErrInvalidProjectStageStatus
		}
	}

	return nil
}

func isEmptyProjectStageUpdate(input models.UpdateProjectStageInput) bool {
	return input.Title == nil &&
		input.Description == nil &&
		input.WeightPercent == nil &&
		input.Status == nil &&
		input.ProgressPercent == nil
}

func isValidProjectStageStatus(status models.ProjectStageStatus) bool {
	switch status {
	case models.ProjectStageStatusPlanned,
		models.ProjectStageStatusInProgress,
		models.ProjectStageStatusDone,
		models.ProjectStageStatusCancelled:
		return true
	default:
		return false
	}
}

func validateProjectStageOrder(
	currentStages []models.ProjectStage,
	items []models.ProjectStageOrderItem,
) error {

	expectedCount := len(currentStages)

	if len(items) != expectedCount {
		return repo.ErrInvalidProjectStageOrder
	}

	if expectedCount == 0 {
		return nil
	}

	currentIDs := make(map[string]struct{}, expectedCount)
	for _, stage := range currentStages {
		id := strings.TrimSpace(stage.ID)
		if id == "" {
			return repo.ErrInvalidProjectStageOrder
		}
		currentIDs[id] = struct{}{}
	}

	seenIDs := make(map[string]struct{}, expectedCount)
	seenPositions := make(map[int32]struct{}, expectedCount)

	for _, item := range items {
		stageID := strings.TrimSpace(item.StageID)
		if stageID == "" {
			return repo.ErrInvalidProjectStageOrder
		}

		if _, ok := currentIDs[stageID]; !ok {
			return repo.ErrInvalidProjectStageOrder
		}

		if _, exists := seenIDs[stageID]; exists {
			return repo.ErrInvalidProjectStageOrder
		}
		seenIDs[stageID] = struct{}{}

		if item.Position < 1 || item.Position > int32(expectedCount) {
			return repo.ErrInvalidProjectStageOrder
		}

		if _, exists := seenPositions[item.Position]; exists {
			return repo.ErrInvalidProjectStageOrder
		}
		seenPositions[item.Position] = struct{}{}
	}

	for position := int32(1); position <= int32(expectedCount); position++ {
		if _, ok := seenPositions[position]; !ok {
			return repo.ErrInvalidProjectStageOrder
		}
	}

	return nil
}
