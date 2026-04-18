package models

type CreateTeamInput struct {
	Name        string
	Description string
	IsInvitable bool
	IsJoinable  bool
	FounderID   string
	LeadID      string
}

type UpdateTeamInput struct {
	TeamID string

	Name        *string
	Description *string
	IsInvitable *bool
	IsJoinable  *bool
	LeadID      *string // nil = не трогаем, "" = очистить

	ActorID string
}

type ListTeamsFilter struct {
	Query     string
	OnlyMy    bool
	ViewerID  string
	PageSize  int32
	PageToken string
}

type ListTeamMembersFilter struct {
	TeamID    string
	PageSize  int32
	PageToken string
}

type UpdateTeamMemberInput struct {
	TeamID string
	UserID string
	Duties *string // nil = не трогаем
}
