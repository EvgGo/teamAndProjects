package models

type ProjectSkillMatchMode int

const (
	ProjectSkillMatchModeUnspecified ProjectSkillMatchMode = 0
	ProjectSkillMatchModeAny         ProjectSkillMatchMode = 1
	ProjectSkillMatchModeAll         ProjectSkillMatchMode = 2
)

type ListProjectsFilter struct {
	TeamID         string
	CreatorID      string
	Status         *ProjectStatus
	OnlyOpen       bool
	Query          string
	ViewerID       string
	PageSize       int32
	PageToken      string
	SkillIDs       []int
	SkillMatchMode ProjectSkillMatchMode
}
