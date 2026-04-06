package models

import (
	"time"
)

type ProjectCursor struct {
	CreatedAt time.Time
	ID        string // UUID в виде строки
}

type ProjectsFilter struct {
	TeamID         string
	CreatorID      string
	Status         string
	UserID         string
	OnlyOpen       bool
	Query          string
	PageSize       int
	Cursor         *ProjectCursor // может быть nil для первой страницы
	SkillIDs       []int
	SkillMatchMode ProjectSkillMatchMode
}
