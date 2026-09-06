package eventmanager

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"encore.dev/beta/errs"
	"encore.app/auth"
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
	var memberExists bool
	if err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM "event_member" WHERE "eventId"=$1 AND "userId"=$2)`,
		p.ID, p.UserID).Scan(&memberExists); err != nil {
		return nil, err
	}
	if !memberExists {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "user is not a member of this event"}
	}
	now := time.Now().UTC()
	if _, err := db.Exec(ctx,
		`INSERT INTO "event_points_entry" (id, "eventId", "userId", points, "createdAt", "updatedAt")
		 VALUES ($1,$2,$3,$4,$5,$5)
		 ON CONFLICT ("eventId", "userId") DO UPDATE SET points=$4, "updatedAt"=$5`,
		"eventpoints-"+newID()[:8], p.ID, p.UserID, p.Points, now); err != nil {
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
		var memberExists bool
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM "event_member" WHERE "eventId"=$1 AND "userId"=$2)`,
			p.ID, m.userID).Scan(&memberExists); err != nil {
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
	if _, err := db.Exec(ctx,
		`UPDATE "event_ladder_entry" SET elo=$1, wins=wins+1 WHERE "eventId"=$2 AND "userId"=$3`,
		updated.WinnerElo, p.ID, p.WinnerID); err != nil {
		return nil, err
	}
	if _, err := db.Exec(ctx,
		`UPDATE "event_ladder_entry" SET elo=$1, losses=losses+1 WHERE "eventId"=$2 AND "userId"=$3`,
		updated.LoserElo, p.ID, p.LoserID); err != nil {
		return nil, err
	}
	return LoadEvent(ctx, p.ID)
}

// getOrCreateLadderElo fetches a ladder entry's elo, creating a 1200-rated
// entry when absent.
func getOrCreateLadderElo(ctx context.Context, eventID, userID string) (int, error) {
	var elo int
	err := db.QueryRow(ctx,
		`SELECT elo FROM "event_ladder_entry" WHERE "eventId"=$1 AND "userId"=$2`,
		eventID, userID).Scan(&elo)
	if errors.Is(err, sql.ErrNoRows) {
		now := time.Now().UTC()
		if _, err := db.Exec(ctx,
			`INSERT INTO "event_ladder_entry" (id, "eventId", "userId", elo, "createdAt")
			 VALUES ($1,$2,$3,1200,$4)
			 ON CONFLICT ("eventId", "userId") DO NOTHING`,
			"ladder-"+newID()[:8], eventID, userID, now); err != nil {
			return 0, err
		}
		if err := db.QueryRow(ctx,
			`SELECT elo FROM "event_ladder_entry" WHERE "eventId"=$1 AND "userId"=$2`,
			eventID, userID).Scan(&elo); err != nil {
			return 0, err
		}
		return elo, nil
	}
	return elo, err
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
		if _, err := db.Exec(ctx, `UPDATE "event" SET tag=$1 WHERE id=$2`, tag, p.ID); err != nil {
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
			if _, err := db.Exec(ctx, `UPDATE "event" SET status=$1, "deletedAt"=$2 WHERE id=$3`, st, now, p.ID); err != nil {
				return nil, err
			}
		} else {
			if _, err := db.Exec(ctx, `UPDATE "event" SET status=$1, "deletedAt"=NULL WHERE id=$2`, st, p.ID); err != nil {
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
