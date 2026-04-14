package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

// ProjectMemberRightsToProto конвертирует права модели в proto-права
func ProjectMemberRightsToProto(rights models.ProjectRights) *workspacev1.ProjectRights {
	return &workspacev1.ProjectRights{
		ManagerRights:   rights.ManagerRights,
		ManagerMember:   rights.ManagerMember,
		ManagerProjects: rights.ManagerProjects,
		ManagerTasks:    rights.ManagerTasks,
	}
}

// ProjectMemberRightsFromProto конвертирует proto-права в модель
func ProjectMemberRightsFromProto(rights *workspacev1.ProjectRights) models.ProjectRights {
	if rights == nil {
		return models.ProjectRights{}
	}
	return models.ProjectRights{
		ManagerRights:   rights.ManagerRights,
		ManagerMember:   rights.ManagerMember,
		ManagerProjects: rights.ManagerProjects,
		ManagerTasks:    rights.ManagerTasks,
	}
}
