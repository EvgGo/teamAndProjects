package repo

import "errors"

var (
	ErrNotFound        = errors.New("repo: not found")
	ErrAlreadyExists   = errors.New("repo: already exists")
	ErrConflict        = errors.New("repo: conflict")
	ErrInvalidInput    = errors.New("repo: invalid input")
	ErrForbidden       = errors.New("repo: forbidden")
	ErrUnauthenticated = errors.New("repo: unauthenticated")
	ErrInternal        = errors.New("internal error")
)
