package models

import "time"

// OptionalTime позволяет различать:
// поле не передали - IsSet=false
// передали NULL - IsSet=true, Value=nil
// передали конкретную дату - IsSet=true, Value!=nil
type OptionalTime struct {
	IsSet bool
	Value *time.Time
}

type UpdateProjectPatch struct {
	Name        *string
	Description *string
	Status      *ProjectStatus
	IsOpen      *bool
	StartedAt   *time.Time
	FinishedAt  OptionalTime

	SkillsSet bool
	SkillIDs  []int32
}

type CreateProjectInput struct {
	TeamID      string
	CreatorID   string
	Name        string
	Description string
	Status      ProjectStatus
	IsOpen      bool
	StartedAt   time.Time
	FinishedAt  *time.Time
	SkillIDs    []int
}
