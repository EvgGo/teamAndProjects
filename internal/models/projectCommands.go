package models

import "time"

// CreateProjectParams вход для сервиса без team_id
type CreateProjectParams struct {
	Name        string
	Description string
	Status      ProjectStatus
	IsOpen      bool

	StartedAt  time.Time
	FinishedAt *time.Time

	TeamName string
	SkillIDs []int
}
