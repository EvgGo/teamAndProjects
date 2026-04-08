package grpc

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func manageableProjectJoinRequestBucketToProto(in *models.ManageableProjectJoinRequestBucket) *workspacev1.ManageableProjectJoinRequestBucket {
	if in == nil {
		return nil
	}

	out := &workspacev1.ManageableProjectJoinRequestBucket{
		ProjectId:            in.ProjectID,
		ProjectName:          in.ProjectName,
		ProjectStatus:        projectStatusFromModel(in.ProjectStatus),
		IsOpen:               in.IsOpen,
		PendingRequestsCount: in.PendingRequestsCount,
		MyRights:             projectRightsToProto(in.MyRights),
	}

	if in.LastRequestCreatedAt != nil {
		out.LastRequestCreatedAt = dateFromTime(*in.LastRequestCreatedAt)
	}

	return out
}

func projectRightsToProto(in models.ProjectRights) *workspacev1.ProjectRights {
	return &workspacev1.ProjectRights{
		ManagerRights:   in.ManagerRights,
		ManagerMember:   in.ManagerMember,
		ManagerProjects: in.ManagerProjects,
		ManagerTasks:    in.ManagerTasks,
	}
}
