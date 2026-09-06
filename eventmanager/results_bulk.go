package eventmanager

import (
	"context"
	"encoding/json"
	"fmt"

	"encore.dev/beta/errs"
	"encore.app/auth"
	"encore.app/eventmanager/sqlc"
)

// Bulk standings endpoints: full-replace (PUT) and additive merge (POST).
//
// Mirrors ts-legacy/eventmanager/results-bulk.ts. POST on the collection path
// is the additive merge, PUT the full replace, matching the codebase
// convention (addEventMember, recordLadderMatch).

// BulkResultsRequest mirrors Replace/MergeRaceResultsParams.
type BulkResultsRequest struct {
	EventID       string            `json:"eventId"`
	RaceID        string            `json:"raceId"`
	Authorization string            `header:"Authorization"`
	Results       []*RaceResultInput `json:"results"`
}

// ValidateResultInputs rejects duplicate userIds and asserts each
// participant exists and is registered for the event/race.
func ValidateResultInputs(ctx context.Context, eventID, raceID string, results []*RaceResultInput) error {
	seen := map[string]bool{}
	for _, entry := range results {
		if seen[entry.UserID] {
			return &errs.Error{Code: errs.InvalidArgument,
				Message: fmt.Sprintf("duplicate userId in payload: %s", entry.UserID)}
		}
		seen[entry.UserID] = true
	}
	for _, entry := range results {
		if err := RequireMembershipForResult(ctx, eventID, raceID, entry.UserID); err != nil {
			return err
		}
	}
	return nil
}

// ReplaceRaceResultsCore makes the payload the complete result set for the
// race: each entry is upserted, existing results absent from the payload are
// deleted, and every affected participant is recomputed.
func ReplaceRaceResultsCore(ctx context.Context, p *BulkResultsRequest) (*RaceResultsResponse, error) {
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	e, err := requireEventRow(ctx, p.EventID)
	if err != nil {
		return nil, err
	}
	race, err := RequireRace(ctx, p.EventID, p.RaceID)
	if err != nil {
		return nil, err
	}
	if err := ValidateResultInputs(ctx, p.EventID, p.RaceID, p.Results); err != nil {
		return nil, err
	}
	mode, custom := scoringRulesOf(e)
	grade := ""
	if race.Grade.Valid {
		grade = race.Grade.String
	}
	payloadIDs := map[string]bool{}
	for _, entry := range p.Results {
		payloadIDs[entry.UserID] = true
		points := resolveEntryPoints(e.ScoringType, mode, custom, grade, entry)
		if _, err := upsertRaceResult(ctx, p.RaceID, entry, points); err != nil {
			return nil, err
		}
	}
	removed, err := removedResultUsers(ctx, p.RaceID, payloadIDs)
	if err != nil {
		return nil, err
	}
	if len(removed) > 0 {
		if err := q().DeleteAbsentRaceResults(ctx, sqlc.DeleteAbsentRaceResultsParams{
			RaceEventId: p.RaceID,
			Column2:     removed,
		}); err != nil {
			return nil, err
		}
	}
	if err := ApplyAutoDeferralsForEvent(ctx, p.EventID, &p.RaceID); err != nil {
		return nil, err
	}
	affected := append(append([]string{}, keysOf(payloadIDs)...), removed...)
	if err := submitCalcForUsers(ctx, p.EventID, affected); err != nil {
		return nil, err
	}
	results, err := ListResultsCore(ctx, p.RaceID)
	if err != nil {
		return nil, err
	}
	return &RaceResultsResponse{Results: results}, nil
}

//encore:api public method=PUT path=/api/race-results-bulk
func ReplaceRaceResults(ctx context.Context, p *BulkResultsRequest) (*RaceResultsResponse, error) {
	return ReplaceRaceResultsCore(ctx, p)
}

// MergeRaceResultsCore upserts each payload entry, leaving results absent
// from the payload untouched.
func MergeRaceResultsCore(ctx context.Context, p *BulkResultsRequest) (*RaceResultsResponse, error) {
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	e, err := requireEventRow(ctx, p.EventID)
	if err != nil {
		return nil, err
	}
	race, err := RequireRace(ctx, p.EventID, p.RaceID)
	if err != nil {
		return nil, err
	}
	if err := ValidateResultInputs(ctx, p.EventID, p.RaceID, p.Results); err != nil {
		return nil, err
	}
	mode, custom := scoringRulesOf(e)
	grade := ""
	if race.Grade.Valid {
		grade = race.Grade.String
	}
	affected := make([]string, 0, len(p.Results))
	for _, entry := range p.Results {
		points := resolveEntryPoints(e.ScoringType, mode, custom, grade, entry)
		if _, err := upsertRaceResult(ctx, p.RaceID, entry, points); err != nil {
			return nil, err
		}
		affected = append(affected, entry.UserID)
	}
	if err := ApplyAutoDeferralsForEvent(ctx, p.EventID, &p.RaceID); err != nil {
		return nil, err
	}
	if err := submitCalcForUsers(ctx, p.EventID, affected); err != nil {
		return nil, err
	}
	results, err := ListResultsCore(ctx, p.RaceID)
	if err != nil {
		return nil, err
	}
	return &RaceResultsResponse{Results: results}, nil
}

//encore:api public method=POST path=/api/race-results-bulk
func MergeRaceResults(ctx context.Context, p *BulkResultsRequest) (*RaceResultsResponse, error) {
	return MergeRaceResultsCore(ctx, p)
}

// scoringRulesOf extracts the scoring mode + parsed custom tables from an event row.
func scoringRulesOf(e *eventRow) (string, any) {
	mode := ""
	if e.ScoringRulesMode.Valid {
		mode = e.ScoringRulesMode.String
	}
	var custom any
	if len(e.CustomScoringTables) > 0 {
		var v any
		if err := json.Unmarshal(e.CustomScoringTables, &v); err == nil {
			custom = v
		}
	}
	return mode, custom
}

// removedResultUsers returns existing result holders absent from the payload.
func removedResultUsers(ctx context.Context, raceID string, payloadIDs map[string]bool) ([]string, error) {
	holders, err := q().ListResultUserIDs(ctx, raceID)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, uid := range holders {
		if !payloadIDs[uid] {
			removed = append(removed, uid)
		}
	}
	return removed, nil
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
