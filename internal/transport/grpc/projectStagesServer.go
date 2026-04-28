package grpc

import (
	"context"
	"strings"

	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"teamAndProjects/internal/models"
	"teamAndProjects/internal/services/svcerr"
	"teamAndProjects/internal/transport/grpc/mapper"
)

// CreateProjectStage создает новый этап проекта
func (s *ProjectsServer) CreateProjectStage(
	ctx context.Context,
	req *workspacev1.CreateProjectStageRequest,
) (*workspacev1.ProjectStage, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	reqLog := s.log.With(
		"grpc_method", "CreateProjectStage",
		"project_id", req.GetProjectId(),
		"position", req.GetPosition(),
		"weight_percent", req.GetWeightPercent(),
		"status", req.GetStatus().String(),
	)

	reqLog.Debug("получен gRPC-запрос на создание этапа проекта")

	stageStatus, ok, err := mapper.ProjectStageStatusToModel(req.GetStatus())
	if err != nil {
		reqLog.Warn("некорректный status этапа", "err", err)
		return nil, status.Error(codes.InvalidArgument, "invalid stage status")
	}

	input := models.CreateProjectStageInput{
		ProjectID:       strings.TrimSpace(req.GetProjectId()),
		Title:           strings.TrimSpace(req.GetTitle()),
		Description:     strings.TrimSpace(req.GetDescription()),
		Position:        req.GetPosition(),
		WeightPercent:   req.GetWeightPercent(),
		ProgressPercent: req.GetProgressPercent(),
	}

	if ok {
		input.Status = stageStatus
	}

	stage, err := s.svc.CreateProjectStage(ctx, input)
	if err != nil {
		reqLog.Warn("не удалось создать этап проекта", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Debug("этап проекта успешно создан", "stage_id", stage.ID)

	return mapper.ProjectStageToProto(&stage), nil
}

// GetProjectStage возвращает один этап проекта
func (s *ProjectsServer) GetProjectStage(
	ctx context.Context,
	req *workspacev1.GetProjectStageRequest,
) (*workspacev1.ProjectStage, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	stageID := strings.TrimSpace(req.GetStageId())

	reqLog := s.log.With(
		"grpc_method", "GetProjectStage",
		"stage_id", stageID,
	)

	reqLog.Debug("получен gRPC-запрос на получение этапа проекта")

	stage, err := s.svc.GetProjectStage(ctx, stageID)
	if err != nil {
		reqLog.Warn("не удалось получить этап проекта", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	return mapper.ProjectStageToProto(&stage), nil
}

// ListProjectStages возвращает список этапов проекта и summary
func (s *ProjectsServer) ListProjectStages(
	ctx context.Context,
	req *workspacev1.ListProjectStagesRequest,
) (*workspacev1.ListProjectStagesResponse, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	projectID := strings.TrimSpace(req.GetProjectId())

	reqLog := s.log.With(
		"grpc_method", "ListProjectStages",
		"project_id", projectID,
	)

	reqLog.Debug("получен gRPC-запрос на список этапов проекта")

	stages, summary, err := s.svc.ListProjectStages(ctx, projectID)
	if err != nil {
		reqLog.Warn("не удалось получить этапы проекта", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	return &workspacev1.ListProjectStagesResponse{
		Stages:  mapper.ProjectStagesToProto(stages),
		Summary: mapper.ProjectStagesSummaryToProto(&summary),
	}, nil
}

// UpdateProjectStage частично обновляет этап проекта.
// position, score, evaluated_by и evaluated_at через этот метод не меняются
func (s *ProjectsServer) UpdateProjectStage(
	ctx context.Context,
	req *workspacev1.UpdateProjectStageRequest,
) (*workspacev1.ProjectStage, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	stageID := strings.TrimSpace(req.GetStageId())

	reqLog := s.log.With(
		"grpc_method", "UpdateProjectStage",
		"stage_id", stageID,
	)

	reqLog.Debug("получен gRPC-запрос на обновление этапа проекта")

	input := models.UpdateProjectStageInput{
		StageID: stageID,
	}

	if req.Title != nil {
		v := strings.TrimSpace(req.GetTitle())
		input.Title = &v
	}

	if req.Description != nil {
		v := strings.TrimSpace(req.GetDescription())
		input.Description = &v
	}

	if req.WeightPercent != nil {
		v := req.GetWeightPercent()
		input.WeightPercent = &v
	}

	if req.Status != nil {
		stageStatus, ok, err := mapper.ProjectStageStatusToModel(req.GetStatus())
		if err != nil {
			reqLog.Warn("некорректный status этапа", "err", err)
			return nil, status.Error(codes.InvalidArgument, "invalid stage status")
		}
		if !ok {
			reqLog.Warn("status этапа не может быть UNSPECIFIED при update")
			return nil, status.Error(codes.InvalidArgument, "stage status must not be unspecified")
		}

		input.Status = &stageStatus
	}

	if req.ProgressPercent != nil {
		v := req.GetProgressPercent()
		input.ProgressPercent = &v
	}

	stage, err := s.svc.UpdateProjectStage(ctx, input)
	if err != nil {
		reqLog.Warn("не удалось обновить этап проекта", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Debug("этап проекта успешно обновлен", "project_id", stage.ProjectID)

	return mapper.ProjectStageToProto(&stage), nil
}

// DeleteProjectStage удаляет этап проекта
func (s *ProjectsServer) DeleteProjectStage(
	ctx context.Context,
	req *workspacev1.DeleteProjectStageRequest,
) (*emptypb.Empty, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	stageID := strings.TrimSpace(req.GetStageId())

	reqLog := s.log.With(
		"grpc_method", "DeleteProjectStage",
		"stage_id", stageID,
	)

	reqLog.Debug("получен gRPC-запрос на удаление этапа проекта")

	if err := s.svc.DeleteProjectStage(ctx, stageID); err != nil {
		reqLog.Warn("не удалось удалить этап проекта", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Debug("этап проекта успешно удален")

	return &emptypb.Empty{}, nil
}

// ReorderProjectStages сохраняет новый порядок этапов проекта.
// Клиент должен передать полный список этапов проекта
func (s *ProjectsServer) ReorderProjectStages(
	ctx context.Context,
	req *workspacev1.ReorderProjectStagesRequest,
) (*workspacev1.ListProjectStagesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	projectID := strings.TrimSpace(req.GetProjectId())

	reqLog := s.log.With(
		"grpc_method", "ReorderProjectStages",
		"project_id", projectID,
		"items_count", len(req.GetItems()),
	)

	reqLog.Debug("получен gRPC-запрос на изменение порядка этапов проекта")

	items := make([]models.ProjectStageOrderItem, 0, len(req.GetItems()))

	for _, item := range req.GetItems() {
		if item == nil {
			return nil, status.Error(codes.InvalidArgument, "order item must not be nil")
		}

		items = append(items, models.ProjectStageOrderItem{
			StageID:  strings.TrimSpace(item.GetStageId()),
			Position: item.GetPosition(),
		})
	}

	stages, summary, err := s.svc.ReorderProjectStages(ctx, projectID, items)
	if err != nil {
		reqLog.Warn("не удалось изменить порядок этапов проекта", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Debug("порядок этапов проекта успешно обновлен")

	return &workspacev1.ListProjectStagesResponse{
		Stages:  mapper.ProjectStagesToProto(stages),
		Summary: mapper.ProjectStagesSummaryToProto(&summary),
	}, nil
}

// EvaluateProjectStage выставляет или обновляет оценку этапа проекта
func (s *ProjectsServer) EvaluateProjectStage(
	ctx context.Context,
	req *workspacev1.EvaluateProjectStageRequest,
) (*workspacev1.ProjectStage, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	stageID := strings.TrimSpace(req.GetStageId())

	reqLog := s.log.With(
		"grpc_method", "EvaluateProjectStage",
		"stage_id", stageID,
		"score", req.GetScore(),
	)

	reqLog.Debug("получен gRPC-запрос на оценку этапа проекта")

	input := models.EvaluateProjectStageInput{
		StageID:      stageID,
		Score:        req.GetScore(),
		ScoreComment: strings.TrimSpace(req.GetScoreComment()),
	}

	stage, err := s.svc.EvaluateProjectStage(ctx, input)
	if err != nil {
		reqLog.Warn("не удалось оценить этап проекта", "err", err)
		return nil, svcerr.ToStatus(err)
	}

	reqLog.Debug("этап проекта успешно оценен", "project_id", stage.ProjectID)

	return mapper.ProjectStageToProto(&stage), nil
}
