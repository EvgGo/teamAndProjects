package models

import "time"

// ProjectStatus - строковый статус проекта в БД
type ProjectStatus string

const (
	ProjectNotStarted ProjectStatus = "not_started"
	ProjectInProgress ProjectStatus = "in_progress"
	ProjectDone       ProjectStatus = "done"
	ProjectOnHold     ProjectStatus = "on_hold"
)

type Project struct {
	ID        string
	TeamID    string
	CreatorID string

	Name        string
	Description string

	Status ProjectStatus
	IsOpen bool

	StartedAt  time.Time
	FinishedAt *time.Time // nil => NULL

	CreatedAt time.Time
	UpdatedAt time.Time
	SkillIDs  []int
	Skills    []ProjectSkill
}

type ProjectRights struct {
	ManagerRights   bool // может выдавать/отнимать права
	ManagerMember   bool // управляет участниками (approve/deny, add/remove)
	ManagerProjects bool // редактирует проект
	ManagerTasks    bool // будет нужно для задач
}

type ProjectMember struct {
	ProjectID string
	UserID    string
	Rights    ProjectRights
}

type ProjectSkill struct {
	ID   int
	Name string
}

type ListProjectMembersFilter struct {
	ProjectID string
	PageSize  int32
	PageToken string
}

type AddProjectMemberInput struct {
	ProjectID string
	UserID    string
	JoinedAt  *time.Time
	Rights    ProjectRights
}

type UpdateProjectMemberRightsInput struct {
	ProjectID string
	UserID    string

	ManagerRights   *bool
	ManagerMember   *bool
	ManagerProjects *bool
	ManagerTasks    *bool
}
