package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

func MyProjectJoinRequestItemToProto(in models.MyProjectJoinRequestItem) *workspacev1.MyProjectJoinRequestItem {
	return &workspacev1.MyProjectJoinRequestItem{
		ProjectId:     in.ProjectID,
		ProjectName:   in.ProjectName,
		ProjectStatus: ProjectStatusFromModel(in.ProjectStatus),
		ProjectIsOpen: in.ProjectIsOpen,
		Request:       ProjectJoinRequestToProto(in.Request),
	}
}

func ProjectJoinRequestToProto(in models.ProjectJoinRequest) *workspacev1.ProjectJoinRequest {

	out := &workspacev1.ProjectJoinRequest{
		Id:          in.ID,
		ProjectId:   in.ProjectID,
		RequesterId: in.RequesterID,
		Message:     in.Message,
		Status:      JoinStatusFromModel(in.Status),
		CreatedAt:   DateFromTime(in.CreatedAt),
	}

	if in.DecidedBy != "" {
		out.DecidedBy = &in.DecidedBy
	}
	if in.DecidedAt != nil {
		out.DecidedAt = DateFromTime(*in.DecidedAt)
	}
	if in.DecisionReason != nil {
		out.DecisionReason = in.DecisionReason
	}

	return out
}
