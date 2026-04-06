package grpc

import (
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/EvgGo/proto/proto/gen/go/teamAndProjects"
)

// dateFromTime делает proto Date из time.Time (UTC, только Y/M/D)
func dateFromTime(t time.Time) *workspacev1.Date {
	t = t.In(time.UTC)
	return &workspacev1.Date{
		Year:  int32(t.Year()),
		Month: int32(t.Month()),
		Day:   int32(t.Day()),
	}
}

// timeFromDate делает time.Time (UTC midnight) из proto Date
// Возвращает (t, isNull, err)
// По договору: year==0 => NULL
func timeFromDate(d *workspacev1.Date) (time.Time, bool, error) {
	if d == nil || d.GetYear() == 0 {
		return time.Time{}, true, nil
	}

	y := int(d.GetYear())
	m := time.Month(d.GetMonth())
	day := int(d.GetDay())

	if y < 1900 || y > 3000 {
		return time.Time{}, false, status.Error(codes.InvalidArgument, "date year out of range")
	}
	if m < 1 || m > 12 {
		return time.Time{}, false, status.Error(codes.InvalidArgument, "date month out of range")
	}
	if day < 1 || day > 31 {
		return time.Time{}, false, status.Error(codes.InvalidArgument, "date day out of range")
	}

	tt := time.Date(y, m, day, 0, 0, 0, 0, time.UTC)

	// Валидация реальной даты
	if tt.Year() != y || tt.Month() != m || tt.Day() != day {
		return time.Time{}, false, status.Error(codes.InvalidArgument, "invalid calendar date")
	}

	return tt, false, nil
}

// dateFromTimePtr делает proto Date из *time.Time.
// Если входной указатель nil, возвращает nil (NULL)
func dateFromTimePtr(t *time.Time) *workspacev1.Date {
	if t == nil {
		return nil
	}
	return dateFromTime(*t)
}
