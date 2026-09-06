package teammanager

import (
	"context"
	"database/sql"
	"errors"

	"encore.dev/beta/errs"
	"encore.app/auth"
	"encore.app/teammanager/sqlc"
)

// --- updateTeamStats (mirrors ts-legacy/teammanager/team-stats.ts) ---
//
// Updates a team's aggregate statistics. Requires a role with organization
// update permission (administrator) in the target team; site admins
// short-circuit via RequirePermission. A nil pointer leaves the column
// unchanged.

// TeamStatsUpdate carries optional stat values.
type TeamStatsUpdate struct {
	RankingAverage        *float64
	PointsAverage         *float64
	SeasonRank            *int32
	AveragePointsPerEvent *float64
}

func updateTeamStats(ctx context.Context, authorization, id string, p *TeamStatsUpdate) (*Team, error) {
	if _, _, err := auth.RequirePermission(ctx, authorization, id, "organization", "update"); err != nil {
		return nil, err
	}
	if _, err := q().OrgIDByID(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "team not found"}
	} else if err != nil {
		return nil, err
	}
	if p.RankingAverage == nil && p.PointsAverage == nil && p.SeasonRank == nil && p.AveragePointsPerEvent == nil {
		return loadTeam(ctx, id)
	}
	var rankingAverage, pointsAverage, averagePointsPerEvent sql.NullFloat64
	if p.RankingAverage != nil {
		rankingAverage = sql.NullFloat64{Float64: *p.RankingAverage, Valid: true}
	}
	if p.PointsAverage != nil {
		pointsAverage = sql.NullFloat64{Float64: *p.PointsAverage, Valid: true}
	}
	if p.AveragePointsPerEvent != nil {
		averagePointsPerEvent = sql.NullFloat64{Float64: *p.AveragePointsPerEvent, Valid: true}
	}
	var seasonRank sql.NullInt32
	if p.SeasonRank != nil {
		seasonRank = sql.NullInt32{Int32: *p.SeasonRank, Valid: true}
	}
	if err := q().UpdateTeamStats(ctx, sqlc.UpdateTeamStatsParams{
		RankingAverage:        rankingAverage,
		PointsAverage:         pointsAverage,
		SeasonRank:            seasonRank,
		AveragePointsPerEvent: averagePointsPerEvent,
		ID:                    id,
	}); err != nil {
		return nil, err
	}
	invalidateTeamCache(ctx, id)
	return loadTeam(ctx, id)
}

// --- HTTP endpoint (thin wrapper over the core above) ---

// UpdateTeamStatsRequest carries the team id, auth header, and stat values.
type UpdateTeamStatsRequest struct {
	ID                    string   `json:"id"`
	Authorization         string   `header:"Authorization"`
	RankingAverage        *float64 `json:"rankingAverage,omitempty"`
	PointsAverage         *float64 `json:"pointsAverage,omitempty"`
	SeasonRank            *int32   `json:"seasonRank,omitempty"`
	AveragePointsPerEvent *float64 `json:"averagePointsPerEvent,omitempty"`
}

//encore:api public method=PATCH path=/api/team-stats
func (s *Service) UpdateTeamStats(ctx context.Context, p *UpdateTeamStatsRequest) (*Team, error) {
	return updateTeamStats(ctx, p.Authorization, p.ID, &TeamStatsUpdate{
		RankingAverage:        p.RankingAverage,
		PointsAverage:         p.PointsAverage,
		SeasonRank:            p.SeasonRank,
		AveragePointsPerEvent: p.AveragePointsPerEvent,
	})
}
