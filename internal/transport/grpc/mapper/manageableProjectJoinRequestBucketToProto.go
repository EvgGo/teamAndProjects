package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func ManageableProjectJoinRequestBucketToProto(in *models.ManageableProjectJoinRequestBucket) *workspacev1.ManageableProjectJoinRequestBucket {

	if in == nil {
		return nil
	}

	out := &workspacev1.ManageableProjectJoinRequestBucket{
		ProjectId:            in.ProjectID,
		ProjectName:          in.ProjectName,
		ProjectStatus:        ProjectStatusFromModel(in.ProjectStatus),
		IsOpen:               in.IsOpen,
		PendingRequestsCount: in.PendingRequestsCount,
		MyRights:             ProjectRightsToProto(in.MyRights),
	}

	if in.LastRequestCreatedAt != nil {
		out.LastRequestCreatedAt = DateFromTime(*in.LastRequestCreatedAt)
	}

	return out
}

func ProjectRightsToProto(in models.ProjectRights) *workspacev1.ProjectRights {

	return &workspacev1.ProjectRights{
		ManagerRights:   in.ManagerRights,
		ManagerMember:   in.ManagerMember,
		ManagerProjects: in.ManagerProjects,
		ManagerTasks:    in.ManagerTasks,
	}
}
