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
)
