package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"strings"
)

func ProjectStatusToProto(status string) workspacev1.ProjectStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "not_started":
		return workspacev1.ProjectStatus_PROJECT_STATUS_NOT_STARTED
	case "in_progress":
		return workspacev1.ProjectStatus_PROJECT_STATUS_IN_PROGRESS
	case "done":
		return workspacev1.ProjectStatus_PROJECT_STATUS_DONE
	case "on_hold":
		return workspacev1.ProjectStatus_PROJECT_STATUS_ON_HOLD
	default:
		return workspacev1.ProjectStatus_PROJECT_STATUS_UNSPECIFIED
	}
}
