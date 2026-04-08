package models

type ListMyProjectJoinRequestsFilter struct {
	ViewerID  string
	Status    *JoinRequestStatus
	PageSize  int32
	PageToken string
}

type MyProjectJoinRequestItem struct {
	ProjectID     string
	ProjectName   string
	ProjectStatus ProjectStatus
	ProjectIsOpen bool

	Request ProjectJoinRequest
}
