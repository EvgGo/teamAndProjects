package models

import "time"

type UpdateProjectInput struct {
	ProjectID string

	Name        *string
	Description *string
	Status      *ProjectStatus
	IsOpen      *bool
	StartedAt   *time.Time

	FinishedAt    *time.Time
	FinishedAtSet bool
	FinishedAtNil bool

	SkillsSet bool
	SkillIDs  []int
}
