package models

import "time"

// JoinRequestStatus - статус заявки на вступление в проект
// В бд хранится как TEXT
type JoinRequestStatus string

const (
	JoinPending   JoinRequestStatus = "pending"
	JoinApproved  JoinRequestStatus = "approved"
	JoinRejected  JoinRequestStatus = "rejected"
	JoinCancelled JoinRequestStatus = "cancelled"
)

type ProjectJoinRequest struct {
	ID          string
	ProjectID   string
	RequesterID string
	Message     string

	Status         JoinRequestStatus
	DecidedBy      string     // "" => NULL
	DecidedAt      *time.Time // nil => NULL
	CreatedAt      time.Time
	DecisionReason *string
}
