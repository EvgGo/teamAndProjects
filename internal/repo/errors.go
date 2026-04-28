package repo

import "errors"

var (
	ErrNotFound               = errors.New("repo: not found")
	ErrAlreadyExists          = errors.New("repo: already exists")
	ErrConflict               = errors.New("repo: conflict")
	ErrInvalidInput           = errors.New("repo: invalid input")
	ErrForbidden              = errors.New("repo: forbidden")
	ErrUnauthenticated        = errors.New("repo: unauthenticated")
	ErrInternal               = errors.New("internal error")
	ErrProjectNotFound        = errors.New("project not found")
	ErrProjectDeleteForbidden = errors.New("only project creator can delete project")
	ErrInvalidCursor          = errors.New("invalid cursor")

	ErrProjectStageNotFound          = errors.New("project stage not found")
	ErrInvalidProjectStageTitle      = errors.New("invalid project stage title")
	ErrInvalidProjectStageStatus     = errors.New("invalid project stage status")
	ErrInvalidProjectStageWeight     = errors.New("invalid project stage weight")
	ErrInvalidProjectStageProgress   = errors.New("invalid project stage progress")
	ErrInvalidProjectStageScore      = errors.New("invalid project stage score")
	ErrProjectStageWeightSumExceeded = errors.New("project stage weight sum exceeded")
	ErrInvalidProjectStageOrder      = errors.New("invalid project stage order")
	ErrProjectStagePositionTaken     = errors.New("project stage position taken")
)
