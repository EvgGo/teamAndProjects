package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"teamAndProjects/internal/models"
)

// JoinRequestToProto конвертирует модель заявки в protobuf-сообщение
func JoinRequestToProto(jrModel models.ProjectJoinRequest) *workspacev1.ProjectJoinRequest {
	pb := &workspacev1.ProjectJoinRequest{
		Id:          jrModel.ID,
		ProjectId:   jrModel.ProjectID,
		RequesterId: jrModel.RequesterID,
		Message:     jrModel.Message,
		Status:      JoinStatusFromModel(jrModel.Status),
		CreatedAt:   DateFromTimePtr(&jrModel.CreatedAt),
	}
	if jrModel.DecidedBy != "" {
		pb.DecidedBy = &jrModel.DecidedBy
	}
	if jrModel.DecidedAt != nil {
		pb.DecidedAt = DateFromTimePtr(jrModel.DecidedAt)
	}
	return pb
}
