package models

import "time"

type ProjectPublic struct {
	ID          string
	TeamID      string
	Name        string
	Description string
	Status      ProjectStatus
	IsOpen      bool
	StartedAt   time.Time
	FinishedAt  *time.Time
	CreatedAt   time.Time
	SkillIDs    []int
	Skills      []ProjectSkill
}

type ListPublicProjectsFilter struct {
	Query          string
	Status         *ProjectStatus
	PageSize       int32
	PageToken      string
	SkillIDs       []int
	SkillMatchMode ProjectSkillMatchMode
	SortBy         ProjectPublicSortBy
	SortOrder      SortOrder
}

type ListPublicProjectsRepoParams struct {
	Query     string
	Status    *ProjectStatus
	PageSize  int32
	PageToken string

	SkillIDs       []int
	SkillMatchMode ProjectSkillMatchMode

	ViewerSkillIDs  []string
	CanComputeMatch bool

	SortBy    ProjectPublicSortBy
	SortOrder SortOrder
}

type PublicProjectRow struct {
	Project                  ProjectPublic
	ProfileSkillMatchPercent *int32
}

type ProjectPublicSortBy string

const (
	ProjectPublicSortByCreatedAt         ProjectPublicSortBy = "created_at"
	ProjectPublicSortByStartedAt         ProjectPublicSortBy = "started_at"
	ProjectPublicSortByProfileSkillMatch ProjectPublicSortBy = "profile_skill_match"
)

type SortOrder string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)
