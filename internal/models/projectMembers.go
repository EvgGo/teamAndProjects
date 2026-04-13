package models

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

type ListProjectMembersParams struct {
	ProjectID string
	PageSize  int32
	PageToken string
}
