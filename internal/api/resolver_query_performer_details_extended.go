package api

import (
	"context"
	"strconv"

	"github.com/stashapp/stash/pkg/models"
)

func (r *queryResolver) PerformerDetailsExtended(ctx context.Context, performerID string) (*models.PerformerDetailsExtendedResult, error) {
	id, err := strconv.Atoi(performerID)
	if err != nil {
		return nil, err
	}

	var stats *models.PerformerDetailsExtendedStats
	var topMale []*models.Performer
	var topFemale []*models.Performer

	if err := r.withReadTxn(ctx, func(ctx context.Context) error {
		var err error
		stats, err = r.repository.Performer.DetailsExtendedStats(ctx, id)
		if err != nil {
			return err
		}

		if len(stats.TopMalePartnerIDs) > 0 {
			topMale, err = r.repository.Performer.FindMany(ctx, stats.TopMalePartnerIDs)
			if err != nil {
				return err
			}
		}

		if len(stats.TopFemalePartnerIDs) > 0 {
			topFemale, err = r.repository.Performer.FindMany(ctx, stats.TopFemalePartnerIDs)
			if err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	var timespan *models.PerformerDetailsExtendedTimespan
	if stats.ScenesTimespan.EarliestDate != nil || stats.ScenesTimespan.LatestDate != nil {
		timespan = &models.PerformerDetailsExtendedTimespan{
			EarliestDate: stats.ScenesTimespan.EarliestDate,
			LatestDate:   stats.ScenesTimespan.LatestDate,
		}
	}

	return &models.PerformerDetailsExtendedResult{
		TopMalePartners:       topMale,
		TopMalePartnerCount:   stats.TopMalePartnerCount,
		TopFemalePartners:     topFemale,
		TopFemalePartnerCount: stats.TopFemalePartnerCount,
		TotalContentTime:      stats.TotalContentTime,
		ScenesTotal:           stats.ScenesTotal,
		ScenesOrganized:       stats.ScenesOrganized,
		ScenesTimespan:        timespan,
	}, nil
}
