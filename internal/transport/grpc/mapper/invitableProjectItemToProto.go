package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func InvitableProjectItemToProto(m models.InvitableProjectItem) *workspacev1.InvitableProjectItem {
	return &workspacev1.InvitableProjectItem{
		ProjectId:     m.ProjectID,
		ProjectName:   m.ProjectName,
		ProjectStatus: ProjectStatusFromModel(m.ProjectStatus),
		IsOpen:        m.IsOpen,
		MyRights:      projectRightsToProto(m.MyRights),
	}
}
