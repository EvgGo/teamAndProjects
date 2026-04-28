package models

import "time"

type ProjectStageStatus string

const (
	ProjectStageStatusPlanned    ProjectStageStatus = "planned"
	ProjectStageStatusInProgress ProjectStageStatus = "in_progress"
	ProjectStageStatusDone       ProjectStageStatus = "done"
	ProjectStageStatusCancelled  ProjectStageStatus = "cancelled"
)

type ProjectStage struct {
	ID        string
	ProjectID string

	Title       string
	Description string

	Position        int32
	WeightPercent   int32
	Status          ProjectStageStatus
	ProgressPercent int32

	Score        *int32
	ScoreComment string

	EvaluatedBy *string
	EvaluatedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProjectStagesSummary struct {
	StagesCount int32

	TotalWeightPercent   int32
	TotalProgressPercent float64

	EvaluatedStagesCount   int32
	EvaluatedWeightPercent int32

	EvaluatedScore *float64
	TotalScore     *float64

	IsTotalScoreReady bool
}

type CreateProjectStageInput struct {
	ProjectID string

	Title       string
	Description string

	Position        int32
	WeightPercent   int32
	Status          ProjectStageStatus
	ProgressPercent int32
}

type UpdateProjectStageInput struct {
	StageID string

	Title       *string
	Description *string

	WeightPercent *int32
	Status        *ProjectStageStatus

	ProgressPercent *int32
}

type ProjectStageOrderItem struct {
	StageID  string
	Position int32
}

type EvaluateProjectStageInput struct {
	StageID string

	Score        int32
	ScoreComment string

	EvaluatedBy string
	EvaluatedAt time.Time
}
