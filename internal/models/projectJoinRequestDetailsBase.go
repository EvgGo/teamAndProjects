package models

import "time"

type ProjectJoinRequestDetailsBase struct {
	ID          string
	ProjectID   string
	RequesterID string

	Message string
	Status  JoinRequestStatus

	DecidedBy *string
	DecidedAt *time.Time
	CreatedAt time.Time

	RejectionReason *string
}

type ListProjectJoinRequestDetailsRepoFilter struct {
	ProjectID string

	Status    *JoinRequestStatus
	PageSize  int32
	PageToken string
}
