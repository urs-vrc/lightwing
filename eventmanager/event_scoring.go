package eventmanager

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"encore.dev/beta/errs"
	"encore.app/auth"
	"encore.app/eventmanager/sqlc"
)

// Event scoring endpoints: points overview, ladder matches, status changes,
// and points recomputation.
//
// Mirrors ts-legacy/eventmanager/event-scoring.ts.

// SetEventPointsRequest mirrors SetPointsParams (PUT /api/events/:id/points/:userId).
type SetEventPointsRequest struct {
	ID            string `json:"id"`
	UserID        string `json:"userId"`
	Authorization string `header:"Authorization"`
	Points        int    `json:"points"`
}

// SetEventPointsCore sets a participant's points on a points-based event.
func SetEventPointsCore(ctx context.Context, p *SetEventPointsRequest) (*EventDetail, error) {
	e, err := requireEventRow(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.ID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	if e.ScoringType != ScoringPoints {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "event is not points-based"}
	}
	memberExists, err := q().EventMemberExists(ctx, sqlc.EventMemberExistsParams{
		EventId: p.ID,
		UserId:  p.UserID,
	})
	if err != nil {
		return nil, err
	}
	if !memberExists {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "user is not a member of this event"}
	}
	now := time.Now().UTC()
	if err := q().UpsertEventPointsEntry(ctx, sqlc.UpsertEventPointsEntryParams{
		ID: "eventpoints-" + newID()[:8], EventId: p.ID, UserId: p.UserID,
		Points: int32(p.Points), CreatedAt: now,
	}); err != nil {
		return nil, err
	}
	return LoadEvent(ctx, p.ID)
}

//encore:api public method=PUT path=/api/event-points
func SetEventPoints(ctx context.Context, p *SetEventPointsRequest) (*EventDetail, error) {
	return SetEventPointsCore(ctx, p)
}

// LadderMatchRequest mirrors LadderMatchParams (POST /api/events/:id/ladder/matches).
type LadderMatchRequest struct {
	ID            string `json:"id"`
	Authorization string `header:"Authorization"`
	WinnerID      string `json:"winnerId"`
	LoserID       string `json:"loserId"`
}

// RecordLadderMatchCore records a 1v1 result on a ladder-elo event and
// updates both ratings.
func RecordLadderMatchCore(ctx context.Context, p *LadderMatchRequest) (*EventDetail, error) {
	e, err := requireEventRow(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.ID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	if e.ScoringType != ScoringLadder {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "event is not ladder-elo"}
	}
	if p.WinnerID == p.LoserID {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "winner and loser must differ"}
	}
	for _, m := range []struct {
		userID string
		role   string
	}{
		{p.WinnerID, "winner"},
		{p.LoserID, "loser"},
	} {
		memberExists, err := q().EventMemberExists(ctx, sqlc.EventMemberExistsParams{
			EventId: p.ID,
			UserId:  m.userID,
		})
		if err != nil {
			return nil, err
		}
		if !memberExists {
			return nil, &errs.Error{Code: errs.FailedPrecondition, Message: m.role + " is not a member of this event"}
		}
	}
	winnerElo, err := getOrCreateLadderElo(ctx, p.ID, p.WinnerID)
	if err != nil {
		return nil, err
	}
	loserElo, err := getOrCreateLadderElo(ctx, p.ID, p.LoserID)
	if err != nil {
		return nil, err
	}
	updated := ComputeElo(winnerElo, loserElo)
	if err := q().RecordLadderWin(ctx, sqlc.RecordLadderWinParams{
		Elo: int32(updated.WinnerElo), EventId: p.ID, UserId: p.WinnerID,
	}); err != nil {
		return nil, err
	}
	if err := q().RecordLadderLoss(ctx, sqlc.RecordLadderLossParams{
		Elo: int32(updated.LoserElo), EventId: p.ID, UserId: p.LoserID,
	}); err != nil {
		return nil, err
	}
	return LoadEvent(ctx, p.ID)
}

// getOrCreateLadderElo fetches a ladder entry's elo, creating a 1200-rated
// entry when absent.
func getOrCreateLadderElo(ctx context.Context, eventID, userID string) (int, error) {
	elo32, err := q().GetLadderElo(ctx, sqlc.GetLadderEloParams{
		EventId: eventID,
		UserId:  userID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		now := time.Now().UTC()
		if err := q().InsertLadderEntry(ctx, sqlc.InsertLadderEntryParams{
			ID: "ladder-" + newID()[:8], EventId: eventID, UserId: userID, CreatedAt: now,
		}); err != nil {
			return 0, err
		}
		elo32, err := q().GetLadderElo(ctx, sqlc.GetLadderEloParams{
			EventId: eventID,
			UserId:  userID,
		})
		if err != nil {
			return 0, err
		}
		return int(elo32), nil
	}
	return int(elo32), err
}

//encore:api public method=POST path=/api/event-ladder-matches
func RecordLadderMatch(ctx context.Context, p *LadderMatchRequest) (*EventDetail, error) {
	return RecordLadderMatchCore(ctx, p)
}

// SetEventStatusRequest mirrors SetStatusParams (PUT /api/events/:id/status).
// Endorsing an event with OFFICIAL tag is reserved for site administrators.
type SetEventStatusRequest struct {
	ID            string  `json:"id"`
	Authorization string  `header:"Authorization"`
	Status        *string `json:"status,omitempty"`
	Tag           *string `json:"tag,omitempty"`
}

// SetEventStatusCore sets an event's lifecycle status and/or hosting tag.
func SetEventStatusCore(ctx context.Context, p *SetEventStatusRequest) (*EventDetail, error) {
	if _, err := requireEventRow(ctx, p.ID); err != nil {
		return nil, err
	}

	if p.Tag != nil && *p.Tag != "" {
		tag := *p.Tag
		if tag == "UNOFFICIAL" {
			tag = "COMMUNITY"
		}
		if tag == "OFFICIAL" {
			if _, err := auth.RequireSiteAdmin(ctx, p.Authorization); err != nil {
				return nil, err
			}
		} else if tag != "COMMUNITY" {
			return nil, &errs.Error{Code: errs.InvalidArgument, Message: "tag must be OFFICIAL or COMMUNITY"}
		} else if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.ID, auth.ActionUpdate); err != nil {
			return nil, err
		}
		if err := q().UpdateEventTag(ctx, sqlc.UpdateEventTagParams{
			Tag: tag, ID: p.ID,
		}); err != nil {
			return nil, err
		}
	}

	if p.Status != nil && *p.Status != "" {
		st := *p.Status
		if st != "DRAFT" && st != "PENDING" && st != "ONGOING" && st != "CONCLUDED" && st != "PENDING_DELETION" {
			return nil, &errs.Error{Code: errs.InvalidArgument, Message: "invalid event status"}
		}
		if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.ID, auth.ActionUpdate); err != nil {
			return nil, err
		}
		if st == "PENDING_DELETION" {
			now := time.Now().UTC()
			if err := q().UpdateEventStatusWithDeletion(ctx, sqlc.UpdateEventStatusWithDeletionParams{
				Status: st, DeletedAt: sql.NullTime{Time: now, Valid: true}, ID: p.ID,
			}); err != nil {
				return nil, err
			}
		} else {
			if err := q().UpdateEventStatus(ctx, sqlc.UpdateEventStatusParams{
				Status: st, ID: p.ID,
			}); err != nil {
				return nil, err
			}
		}
	}

	return LoadEvent(ctx, p.ID)
}

//encore:api public method=PUT path=/api/event-status
func SetEventStatus(ctx context.Context, p *SetEventStatusRequest) (*EventDetail, error) {
	return SetEventStatusCore(ctx, p)
}

// RecomputePointsRequest mirrors RecomputePointsParams.
type RecomputePointsRequest struct {
	ID            string `json:"id"`
	Authorization string `header:"Authorization"`
}

// RecomputePointsResponse reports success.
type RecomputePointsResponse struct {
	Success bool `json:"success"`
}

// RecomputeEventPointsCore recomputes all points-based results for an event.
func RecomputeEventPointsCore(ctx context.Context, p *RecomputePointsRequest) (*RecomputePointsResponse, error) {
	if _, err := requireEventRow(ctx, p.ID); err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.ID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	if err := recomputeEventPoints(ctx, p.ID); err != nil {
		return nil, err
	}
	return &RecomputePointsResponse{Success: true}, nil
}

//encore:api public method=POST path=/api/event-recompute-points
func RecomputeEventPoints(ctx context.Context, p *RecomputePointsRequest) (*RecomputePointsResponse, error) {
	return RecomputeEventPointsCore(ctx, p)
}
