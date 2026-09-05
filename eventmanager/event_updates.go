package eventmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"encore.dev/beta/errs"
	"encore.app/auth"
	"encore.app/scorecalc"
)

// UpdateEventRequest carries the editable fields plus the auth header. The
// event id travels in the body (rather than a :id path param) because this
// Encore version only accepts scalar params alongside path params.
//
// Mirrors ts-legacy/eventmanager/event-updates.ts UpdateEventParams
// (PATCH /api/events/:id). Scoring type is immutable once set.
type UpdateEventRequest struct {
	ID                              string          `json:"id"`
	Authorization                   string          `header:"Authorization"`
	Name                            *string         `json:"name,omitempty"`
	Description                     OptString       `json:"description,omitempty"`
	Tag                             *string         `json:"tag,omitempty"`
	ScoringRulesMode                *string         `json:"scoringRulesMode,omitempty"`
	CustomScoringTables             json.RawMessage `json:"customScoringTables,omitempty"`
	ClassRestriction                OptString       `json:"classRestriction,omitempty"`
	ScheduledAt                     OptString       `json:"scheduledAt,omitempty"`
	ParticipantLimit                OptInt          `json:"participantLimit,omitempty"`
	MaxConcurrentRaceParticipations OptInt          `json:"maxConcurrentRaceParticipations,omitempty"`
}

// jsonEqual reports whether two JSON documents are semantically equal.
func jsonEqual(a, b []byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return string(a) == string(b)
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	ca, err := json.Marshal(va)
	if err != nil {
		return false
	}
	cb, err := json.Marshal(vb)
	if err != nil {
		return false
	}
	return string(ca) == string(cb)
}

// UpdateEventCore updates an event's editable fields and returns its detail.
func UpdateEventCore(ctx context.Context, p *UpdateEventRequest) (*EventDetail, error) {
	existing, err := scanEventRow(db.QueryRow(ctx,
		`SELECT `+eventColumns+` FROM "event" WHERE id = $1`, p.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "event not found"}
	}
	if err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.ID, auth.ActionUpdate); err != nil {
		return nil, err
	}

	isGranular := existing.GranularParticipation
	var participantLimit, maxConcurrent *int
	var clearParticipantLimit, clearMaxConcurrent bool
	if p.ParticipantLimit.Set {
		if p.ParticipantLimit.Value == nil {
			clearParticipantLimit = true
		} else {
			participantLimit = p.ParticipantLimit.Value
		}
	}
	if p.MaxConcurrentRaceParticipations.Set {
		if p.MaxConcurrentRaceParticipations.Value == nil {
			clearMaxConcurrent = true
		} else {
			maxConcurrent = p.MaxConcurrentRaceParticipations.Value
		}
	}

	if isGranular {
		if participantLimit != nil {
			return nil, &errs.Error{Code: errs.InvalidArgument, Message: "Granular events cannot have an event-level participant limit"}
		}
	} else {
		if maxConcurrent != nil {
			return nil, &errs.Error{Code: errs.InvalidArgument, Message: "Regular events cannot have a maxConcurrentRaceParticipations limit"}
		}
	}

	// Capacity checks before reducing a limit.
	if !isGranular && participantLimit != nil {
		var currentCount int
		if err := db.QueryRow(ctx,
			`SELECT COUNT(*) FROM "event_member" WHERE "eventId" = $1`, p.ID,
		).Scan(&currentCount); err != nil {
			return nil, err
		}
		if err := AssertLimitCanBeReduced(currentCount, *participantLimit,
			CodeParticipantLimitBelowEnrollment,
			"Participant limit cannot be lower than the current enrollment"); err != nil {
			return nil, err
		}
	}
	if isGranular && maxConcurrent != nil {
		var maxJoined int
		if err := db.QueryRow(ctx,
			`SELECT COALESCE(MAX(c), 0) FROM (
			   SELECT COUNT(*) AS c FROM "race_event_member" m
			   JOIN "race_event" r ON r.id = m."raceEventId"
			   WHERE r."eventId" = $1 GROUP BY m."userId"
			 ) t`, p.ID,
		).Scan(&maxJoined); err != nil {
			return nil, err
		}
		if err := AssertLimitCanBeReduced(maxJoined, *maxConcurrent,
			CodeParticipantLimitBelowEnrollment,
			"Max races limit cannot be lower than any member's current race enrollment count"); err != nil {
			return nil, err
		}
	}

	var updatedScoringRulesMode *string
	var updatedCustomTables []byte
	var clearCustomTables bool
	triggerRecomputation := false

	if existing.ScoringType == ScoringPoints {
		if p.ScoringRulesMode != nil {
			mode := *p.ScoringRulesMode
			if mode == "" {
				mode = "STANDARD"
			}
			if mode == "CUSTOM" {
				tables := p.CustomScoringTables
				if len(tables) == 0 {
					tables = existing.CustomScoringTables
				}
				if len(tables) == 0 || string(tables) == "null" {
					return nil, &errs.Error{Code: errs.InvalidArgument, Message: "customScoringTables is required when scoringRulesMode is CUSTOM"}
				}
				var v any
				if err := json.Unmarshal(tables, &v); err != nil {
					return nil, &errs.Error{Code: errs.InvalidArgument, Message: "customScoringTables must be an object"}
				}
				if _, err := ValidateCustomScoringTables(v); err != nil {
					return nil, err
				}
				updatedCustomTables = tables
			} else {
				clearCustomTables = true
			}
			updatedScoringRulesMode = &mode
			if !existing.ScoringRulesMode.Valid || existing.ScoringRulesMode.String != mode {
				triggerRecomputation = true
			}
		}
		if len(p.CustomScoringTables) > 0 {
			effectiveMode := ""
			if updatedScoringRulesMode != nil {
				effectiveMode = *updatedScoringRulesMode
			} else if existing.ScoringRulesMode.Valid {
				effectiveMode = existing.ScoringRulesMode.String
			}
			if effectiveMode == "CUSTOM" {
				var v any
				if err := json.Unmarshal(p.CustomScoringTables, &v); err != nil {
					return nil, &errs.Error{Code: errs.InvalidArgument, Message: "customScoringTables must be an object"}
				}
				if _, err := ValidateCustomScoringTables(v); err != nil {
					return nil, err
				}
				updatedCustomTables = p.CustomScoringTables
				clearCustomTables = false
				if !jsonEqual(existing.CustomScoringTables, updatedCustomTables) {
					triggerRecomputation = true
				}
			}
		}
	}

	// Build the dynamic UPDATE.
	type setClause struct {
		clause string
		arg    any
	}
	var sets []setClause
	if p.Name != nil {
		sets = append(sets, setClause{"name = ?", truncate(*p.Name, 255)})
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
		}
		sets = append(sets, setClause{"tag = ?", tag})
	}
	if p.Description.Set {
		var v any
		if p.Description.Value != nil {
			v = *p.Description.Value
		}
		sets = append(sets, setClause{"description = ?", v})
	}
	if updatedScoringRulesMode != nil {
		sets = append(sets, setClause{"\"scoringRulesMode\" = ?", *updatedScoringRulesMode})
	}
	if updatedCustomTables != nil {
		sets = append(sets, setClause{"\"customScoringTables\" = ?", string(updatedCustomTables)})
	} else if clearCustomTables {
		sets = append(sets, setClause{"\"customScoringTables\" = ?", nil})
	}
	if p.ClassRestriction.Set {
		var v any
		if p.ClassRestriction.Value != nil && *p.ClassRestriction.Value != "" {
			v = *p.ClassRestriction.Value
		}
		sets = append(sets, setClause{"\"classRestriction\" = ?", v})
	}
	if p.ScheduledAt.Set {
		var v any
		if p.ScheduledAt.Value != nil && *p.ScheduledAt.Value != "" {
			t, err := time.Parse(time.RFC3339Nano, *p.ScheduledAt.Value)
			if err != nil {
				return nil, &errs.Error{Code: errs.InvalidArgument, Message: "scheduledAt must be an ISO-8601 timestamp"}
			}
			utc := t.UTC()
			v = utc
		}
		sets = append(sets, setClause{"\"scheduledAt\" = ?", v})
	}
	if participantLimit != nil {
		sets = append(sets, setClause{"\"participantLimit\" = ?", *participantLimit})
	} else if clearParticipantLimit {
		sets = append(sets, setClause{"\"participantLimit\" = ?", nil})
	}
	if maxConcurrent != nil {
		sets = append(sets, setClause{"\"maxConcurrentRaceParticipations\" = ?", *maxConcurrent})
	} else if clearMaxConcurrent {
		sets = append(sets, setClause{"\"maxConcurrentRaceParticipations\" = ?", nil})
	}

	now := time.Now().UTC()
	if len(sets) > 0 {
		query := `UPDATE "event" SET "updatedAt" = $1`
		args := []any{now}
		for _, s := range sets {
			// Replace the ? placeholder with the next positional arg.
			clause := ""
			for i := 0; i < len(s.clause); i++ {
				if s.clause[i] == '?' {
					args = append(args, s.arg)
					clause += "$" + strconv.Itoa(len(args))
				} else {
					clause += string(s.clause[i])
				}
			}
			query += ", " + clause
		}
		args = append(args, p.ID)
		query += " WHERE id = $" + strconv.Itoa(len(args))
		if _, err := db.Exec(ctx, query, args...); err != nil {
			return nil, err
		}
	}

	if triggerRecomputation {
		if err := recomputeEventPoints(ctx, p.ID); err != nil {
			return nil, err
		}
	}
	return LoadEvent(ctx, p.ID)
}

// storedResultRow is a race-result row joined with its race grade.
type storedResultRow struct {
	ID           string
	UserID       string
	Position     sql.NullInt64
	Points       int
	ResultStatus sql.NullString
	Grade        sql.NullString
}

// resolveResultPoints maps a stored race-result row to points under event rules.
func resolveResultPoints(mode string, custom any, r *storedResultRow) int {
	var position *int
	if r.Position.Valid {
		n := int(r.Position.Int64)
		position = &n
	}
	status := ""
	if r.ResultStatus.Valid {
		status = r.ResultStatus.String
	}
	grade := ""
	if r.Grade.Valid {
		grade = r.Grade.String
	}
	return ResolvePoints(mode, custom, grade, position, status)
}

// recomputeEventPoints recalculates every race result's points under the
// event's current scoring rules and queues a scorecalc job for affected
// users. Idempotent and safe to call repeatedly.
//
// Mirrors ts-legacy/eventmanager/events.ts recomputeEventPointsInternal
// (minus the results-owned auto-deferral pass, which lives with results.ts).
func recomputeEventPoints(ctx context.Context, eventID string) error {
	var scoringType int
	var scoringRulesMode sql.NullString
	var customTables []byte
	err := db.QueryRow(ctx,
		`SELECT "scoringType", "scoringRulesMode", "customScoringTables" FROM "event" WHERE id = $1`,
		eventID,
	).Scan(&scoringType, &scoringRulesMode, &customTables)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if scoringType != ScoringPoints {
		return nil
	}
	mode := ""
	if scoringRulesMode.Valid {
		mode = scoringRulesMode.String
	}
	var custom any
	if len(customTables) > 0 {
		var v any
		if err := json.Unmarshal(customTables, &v); err == nil {
			custom = v
		}
	}

	rows, err := db.Query(ctx,
		`SELECT res.id, res."userId", res.position, res.points, res."resultStatus", r.grade
		 FROM "race_result" res JOIN "race_event" r ON r.id = res."raceEventId"
		 WHERE r."eventId" = $1`, eventID)
	if err != nil {
		return err
	}
	defer rows.Close()
	affected := map[string]bool{}
	var stale []storedResultRow
	for rows.Next() {
		var r storedResultRow
		if err := rows.Scan(&r.ID, &r.UserID, &r.Position, &r.Points, &r.ResultStatus, &r.Grade); err != nil {
			return err
		}
		calculated := resolveResultPoints(mode, custom, &r)
		if r.Points != calculated {
			stale = append(stale, r)
		}
		affected[r.UserID] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range stale {
		calculated := resolveResultPoints(mode, custom, &r)
		if _, err := db.Exec(ctx,
			`UPDATE "race_result" SET points = $1 WHERE id = $2`, calculated, r.ID); err != nil {
			return err
		}
	}
	if len(affected) > 0 {
		userIDs := make([]string, 0, len(affected))
		for u := range affected {
			userIDs = append(userIDs, u)
		}
		if _, err := scorecalc.SubmitCalc(ctx, &scorecalc.SubmitCalcParams{
			EventID: eventID, UserIDs: userIDs,
		}); err != nil {
			return err
		}
	}
	return nil
}

//encore:api public method=PATCH path=/api/events
func UpdateEvent(ctx context.Context, p *UpdateEventRequest) (*EventDetail, error) {
	return UpdateEventCore(ctx, p)
}
