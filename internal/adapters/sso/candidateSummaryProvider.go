package sso

import (
	"context"
	ssov1 "github.com/EvgGo/proto/proto/gen/go/sso"
	"log/slog"
	"strings"
	"teamAndProjects/internal/helpers"
	"teamAndProjects/internal/models"
	"teamAndProjects/pkg/utils"
)

type CandidateSummaryProvider interface {
	GetCandidatePublicSummaries(
		ctx context.Context,
		userIDs []string,
	) (map[string]models.CandidatePublicSummary, error)
}

type CandidateSummaryProviderSSO struct {
	client ssov1.UserProfileClient
	log    *slog.Logger
}

func NewSSOCandidateSummaryProvider(
	log *slog.Logger,
	client ssov1.UserProfileClient,
) *CandidateSummaryProviderSSO {
	return &CandidateSummaryProviderSSO{
		client: client,
		log:    log,
	}
}

func (p *CandidateSummaryProviderSSO) GetCandidatePublicSummaries(
	ctx context.Context,
	userIDs []string,
) (map[string]models.CandidatePublicSummary, error) {

	reqLog := p.log.With(
		"provider_method", "GetCandidatePublicSummaries",
		"user_ids_count", len(userIDs),
	)

	uniqueIDs := utils.UniqueNonEmptyStrings(userIDs)
	if len(uniqueIDs) == 0 {
		reqLog.Debug("пустой список user_ids")
		return map[string]models.CandidatePublicSummary{}, nil
	}

	reqLog.Debug("запрашиваем публичные профили кандидатов из sso")

	ctx = helpers.ForwardAuthorization(ctx)

	resp, err := p.client.GetProfilesByIds(ctx, &ssov1.GetProfilesByIdsRequest{
		UserIds: uniqueIDs,
	})
	if err != nil {
		reqLog.Warn("не удалось получить публичные профили кандидатов из sso", "err", err)
		return nil, err
	}

	out := make(map[string]models.CandidatePublicSummary, len(resp.GetUsers()))

	for _, user := range resp.GetUsers() {
		if user == nil {
			continue
		}

		userID := strings.TrimSpace(user.GetId())
		if userID == "" {
			reqLog.Warn("sso вернул пользователя с пустым id, запись пропущена")
			continue
		}

		about := strings.TrimSpace(user.GetAbout())

		out[userID] = models.CandidatePublicSummary{
			UserID:    userID,
			FirstName: strings.TrimSpace(user.GetFirstName()),
			LastName:  strings.TrimSpace(user.GetLastName()),
			About:     &about,
			Skills:    mapSSOSkillsToProjectSkills(user.GetSkills()),
		}
	}

	reqLog.Debug(
		"публичные summary кандидатов успешно получены",
		"requested_count", len(uniqueIDs),
		"loaded_count", len(out),
	)

	return out, nil
}

func mapSSOSkillsToProjectSkills(items []*ssov1.Skill) []models.Skill {

	if len(items) == 0 {
		return []models.Skill{}
	}

	out := make([]models.Skill, 0, len(items))
	seen := make(map[string]struct{}, len(items))

	for _, item := range items {
		if item == nil {
			continue
		}

		id := strings.TrimSpace(item.GetId())
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		out = append(out, models.Skill{
			ID:   id,
			Name: strings.TrimSpace(item.GetName()),
		})
	}

	return out
}
