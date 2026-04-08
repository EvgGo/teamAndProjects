package models

import "time"

type ListManageableProjectJoinRequestBucketsFilter struct {
	ViewerID  string
	Status    JoinRequestStatus
	Query     string
	PageSize  int32
	PageToken string
}

type ManageableProjectJoinRequestBucket struct {
	ProjectID            string
	ProjectName          string
	ProjectStatus        ProjectStatus
	IsOpen               bool
	PendingRequestsCount int32
	LastRequestCreatedAt *time.Time
	MyRights             ProjectRights
}
