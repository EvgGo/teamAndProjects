package sso

import (
	"context"
	ssov1 "github.com/EvgGo/proto/proto/gen/go/sso"
	"google.golang.org/protobuf/types/known/emptypb"
	"log/slog"
	"strings"
	"teamAndProjects/internal/helpers"
	"teamAndProjects/internal/models"
)

type MyAssessmentResultsProviderSSO struct {
	client ssov1.UserProfileClient
	log    *slog.Logger
}

func NewMyAssessmentResultsProviderSSO(
	client ssov1.UserProfileClient,
	log *slog.Logger,
) *MyAssessmentResultsProviderSSO {
	return &MyAssessmentResultsProviderSSO{
		client: client,
		log:    log,
	}
}

func (p *MyAssessmentResultsProviderSSO) GetMySavedAssessmentResults(
	ctx context.Context,
) (map[int64]models.SavedAssessmentResult, error) {

	reqLog := p.log.With("provider_method", "GetMySavedAssessmentResults")

	ctx = helpers.ForwardAuthorization(ctx)

	resp, err := p.client.GetMe(ctx, &emptypb.Empty{})
	if err != nil {
		reqLog.Warn("не удалось получить GetMe из sso", "err", err)
		return nil, err
	}

	out := make(map[int64]models.SavedAssessmentResult, len(resp.GetSavedAssessmentResults()))

	for _, item := range resp.GetSavedAssessmentResults() {
		if item == nil {
			continue
		}
		if item.GetAssessmentId() <= 0 {
			continue
		}

		var mode models.ProjectAssessmentMode
		switch item.GetMode() {
		case ssov1.SavedAssessmentMode_SAVED_ASSESSMENT_MODE_SUBTOPIC:
			mode = models.ProjectAssessmentModeSubtopic
		case ssov1.SavedAssessmentMode_SAVED_ASSESSMENT_MODE_GLOBAL:
			mode = models.ProjectAssessmentModeGlobal
		default:
			mode = models.ProjectAssessmentModeUnspecified
		}

		out[item.GetAssessmentId()] = models.SavedAssessmentResult{
			AssessmentID: item.GetAssessmentId(),
			Level:        item.GetLevel(),
			Mode:         mode,
		}
	}

	reqLog.Debug(
		"результаты тестов текущего пользователя успешно получены",
		"user_id", strings.TrimSpace(resp.GetId()),
		"results_count", len(out),
	)

	return out, nil
}
