package mapper

import (
	workspacev1 "github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
	"time"
)

func TimeToDateProto(t time.Time) *workspacev1.Date {
	tt := t.UTC()
	return &workspacev1.Date{
		Year:  int32(tt.Year()),
		Month: int32(tt.Month()),
		Day:   int32(tt.Day()),
	}
}
