package models

import "time"

type CreateProjectTeamMode string

const (
	CreateProjectTeamModeUnspecified          CreateProjectTeamMode = "unspecified"
	CreateProjectTeamModeAutoGenerate         CreateProjectTeamMode = "auto_generate"
	CreateProjectTeamModeAttachExistingByName CreateProjectTeamMode = "attach_existing_by_name"
	CreateProjectTeamModeCreateNewWithName    CreateProjectTeamMode = "create_new_with_name"
)

// CreateProjectParams вход для сервиса без team_id
type CreateProjectParams struct {
	Name        string
	Description string
	Status      ProjectStatus
	IsOpen      bool

	StartedAt  time.Time
	FinishedAt *time.Time

	TeamName string
	TeamMode CreateProjectTeamMode

	SkillIDs []int
}
