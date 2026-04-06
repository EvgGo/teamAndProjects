package projectsvc

import (
	"github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func ProjectRightsFromProto(r *workspacev1.ProjectRights) models.ProjectRights {
	if r == nil {
		return models.ProjectRights{}
	}
	return models.ProjectRights{
		ManagerRights:   r.GetManagerRights(),
		ManagerMember:   r.GetManagerMember(),
		ManagerProjects: r.GetManagerProjects(),
		ManagerTasks:    r.GetManagerTasks(),
	}
}
