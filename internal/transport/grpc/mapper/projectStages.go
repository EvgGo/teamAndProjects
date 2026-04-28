package mapper

import (
	"fmt"
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"

	"teamAndProjects/internal/models"
)

// ProjectStageStatusToModel переводит proto enum ProjectStageStatus в доменный статус.
// UNSPECIFIED возвращает ok=false, чтобы create мог применить default planned,
// а update мог считать явно переданный UNSPECIFIED невалидным значением
func ProjectStageStatusToModel(
	status workspacev1.ProjectStageStatus,
) (models.ProjectStageStatus, bool, error) {
	switch status {
	case workspacev1.ProjectStageStatus_PROJECT_STAGE_STATUS_UNSPECIFIED:
		return "", false, nil
	case workspacev1.ProjectStageStatus_PROJECT_STAGE_STATUS_PLANNED:
		return models.ProjectStageStatusPlanned, true, nil
	case workspacev1.ProjectStageStatus_PROJECT_STAGE_STATUS_IN_PROGRESS:
		return models.ProjectStageStatusInProgress, true, nil
	case workspacev1.ProjectStageStatus_PROJECT_STAGE_STATUS_DONE:
		return models.ProjectStageStatusDone, true, nil
	case workspacev1.ProjectStageStatus_PROJECT_STAGE_STATUS_CANCELLED:
		return models.ProjectStageStatusCancelled, true, nil
	default:
		return "", false, fmt.Errorf("invalid project stage status: %v", status)
	}
}

// ProjectStageStatusToProto переводит доменный статус этапа в proto enum
func ProjectStageStatusToProto(status models.ProjectStageStatus) workspacev1.ProjectStageStatus {
	switch status {
	case models.ProjectStageStatusPlanned:
		return workspacev1.ProjectStageStatus_PROJECT_STAGE_STATUS_PLANNED
	case models.ProjectStageStatusInProgress:
		return workspacev1.ProjectStageStatus_PROJECT_STAGE_STATUS_IN_PROGRESS
	case models.ProjectStageStatusDone:
		return workspacev1.ProjectStageStatus_PROJECT_STAGE_STATUS_DONE
	case models.ProjectStageStatusCancelled:
		return workspacev1.ProjectStageStatus_PROJECT_STAGE_STATUS_CANCELLED
	default:
		return workspacev1.ProjectStageStatus_PROJECT_STAGE_STATUS_UNSPECIFIED
	}
}

// ProjectStageToProto переводит доменную модель этапа проекта в proto
func ProjectStageToProto(stage *models.ProjectStage) *workspacev1.ProjectStage {
	if stage == nil {
		return nil
	}

	out := &workspacev1.ProjectStage{
		Id:              stage.ID,
		ProjectId:       stage.ProjectID,
		Title:           stage.Title,
		Description:     stage.Description,
		Position:        stage.Position,
		WeightPercent:   stage.WeightPercent,
		Status:          ProjectStageStatusToProto(stage.Status),
		ProgressPercent: stage.ProgressPercent,
		ScoreComment:    stage.ScoreComment,
		CreatedAt:       DateFromTime(stage.CreatedAt),
		UpdatedAt:       DateFromTime(stage.UpdatedAt),
	}

	if stage.Score != nil {
		v := *stage.Score
		out.Score = &v
	}

	if stage.EvaluatedBy != nil {
		v := *stage.EvaluatedBy
		out.EvaluatedBy = &v
	}

	if stage.EvaluatedAt != nil {
		out.EvaluatedAt = DateFromTime(*stage.EvaluatedAt)
	}

	return out
}

// ProjectStagesToProto переводит список доменных этапов проекта в proto
func ProjectStagesToProto(stages []models.ProjectStage) []*workspacev1.ProjectStage {
	if len(stages) == 0 {
		return nil
	}

	out := make([]*workspacev1.ProjectStage, 0, len(stages))
	for i := range stages {
		out = append(out, ProjectStageToProto(&stages[i]))
	}

	return out
}

// ProjectStagesSummaryToProto переводит рассчитанную summary этапов проекта в proto
func ProjectStagesSummaryToProto(summary *models.ProjectStagesSummary) *workspacev1.ProjectStagesSummary {
	if summary == nil {
		return nil
	}

	out := &workspacev1.ProjectStagesSummary{
		StagesCount:            summary.StagesCount,
		TotalWeightPercent:     summary.TotalWeightPercent,
		TotalProgressPercent:   summary.TotalProgressPercent,
		EvaluatedStagesCount:   summary.EvaluatedStagesCount,
		EvaluatedWeightPercent: summary.EvaluatedWeightPercent,
		IsTotalScoreReady:      summary.IsTotalScoreReady,
	}

	if summary.EvaluatedScore != nil {
		v := *summary.EvaluatedScore
		out.EvaluatedScore = &v
	}

	if summary.TotalScore != nil {
		v := *summary.TotalScore
		out.TotalScore = &v
	}

	return out
}
