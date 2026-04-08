package sso

import (
	"context"
	ssov1 "github.com/EvgGo/proto/proto/gen/go/sso"
	"log/slog"
	"strconv"
	"strings"
	"teamAndProjects/internal/helpers"
	"teamAndProjects/internal/models"
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

	uniqueIDs := uniqueNonEmptyStrings(userIDs)
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

		out[userID] = models.CandidatePublicSummary{
			UserID:    userID,
			FirstName: strings.TrimSpace(user.GetFirstName()),
			LastName:  strings.TrimSpace(user.GetLastName()),
			About:     strings.TrimSpace(user.GetAbout()),
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

func mapSSOSkillsToProjectSkills(items []*ssov1.Skill) []models.ProjectSkill {

	if len(items) == 0 {
		return []models.ProjectSkill{}
	}

	out := make([]models.ProjectSkill, 0, len(items))
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

		idInt, err := strconv.Atoi(id)
		if err != nil {
			continue
		}

		out = append(out, models.ProjectSkill{
			ID:   idInt,
			Name: strings.TrimSpace(item.GetName()),
		})
	}

	return out
}

func uniqueNonEmptyStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))

	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}

	return out
}
