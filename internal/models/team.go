package models

import "time"

// Team членство в команде появляться автоматически,
// когда пользователь становится участником проекта команды
type Team struct {
	ID          string
	Name        string
	Description string

	IsInvitable bool
	IsJoinable  bool

	FounderID string
	LeadID    string // "" => NULL

	CreatedAt    time.Time
	UpdatedAt    time.Time
	MyRights     TeamRights
	Capabilities TeamCapabilities
}

type TeamMember struct {
	TeamID   string
	UserID   string
	Duties   string
	JoinedAt time.Time
	Rights   TeamRights
}
