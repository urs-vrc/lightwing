package eventmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"encore.dev/beta/errs"
	"encore.app/auth"
	"encore.app/eventmanager/sqlc"
	"encore.app/scorecalc"

	"github.com/sqlc-dev/pqtype"
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
	existing, err := requireEventRow(ctx, p.ID)
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
		currentCount64, err := q().GetEventMemberCount(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		if err := AssertLimitCanBeReduced(int(currentCount64), *participantLimit,
			CodeParticipantLimitBelowEnrollment,
			"Participant limit cannot be lower than the current enrollment"); err != nil {
			return nil, err
		}
	}
	if isGranular && maxConcurrent != nil {
		maxJoined, err := q().GetMaxRaceEnrollment(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		if err := AssertLimitCanBeReduced(int(maxJoined), *maxConcurrent,
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

	// Static UPDATE with tri-state params (see UpdateEvent query): set flags
	// distinguish "leave unchanged" from "clear to NULL".
	params := sqlc.UpdateEventParams{UpdatedAt: time.Now().UTC(), ID: p.ID}
	hasUpdate := false
	if p.Name != nil {
		params.Name = sql.NullString{String: truncate(*p.Name, 255), Valid: true}
		hasUpdate = true
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
		params.Tag = sql.NullString{String: tag, Valid: true}
		hasUpdate = true
	}
	if p.Description.Set {
		params.DescSet = true
		params.DescVal = nullStringFromPtr(p.Description.Value)
		hasUpdate = true
	}
	if updatedScoringRulesMode != nil {
		params.Mode = sql.NullString{String: *updatedScoringRulesMode, Valid: true}
		hasUpdate = true
	}
	if updatedCustomTables != nil {
		params.CtSet = true
		params.CtVal = pqtype.NullRawMessage{RawMessage: updatedCustomTables, Valid: true}
		hasUpdate = true
	} else if clearCustomTables {
		params.CtClear = true
		hasUpdate = true
	}
	if p.ClassRestriction.Set {
		params.ClassSet = true
		if p.ClassRestriction.Value != nil && *p.ClassRestriction.Value != "" {
			params.ClassVal = *p.ClassRestriction.Value
		}
		hasUpdate = true
	}
	if p.ScheduledAt.Set {
		params.SchedSet = true
		if p.ScheduledAt.Value != nil && *p.ScheduledAt.Value != "" {
			t, err := time.Parse(time.RFC3339Nano, *p.ScheduledAt.Value)
			if err != nil {
				return nil, &errs.Error{Code: errs.InvalidArgument, Message: "scheduledAt must be an ISO-8601 timestamp"}
			}
			utc := t.UTC()
			params.SchedVal = sql.NullTime{Time: utc, Valid: true}
		}
		hasUpdate = true
	}
	if participantLimit != nil {
		params.PlVal = sql.NullInt32{Int32: int32(*participantLimit), Valid: true}
		hasUpdate = true
	} else if clearParticipantLimit {
		params.PlClear = true
		hasUpdate = true
	}
	if maxConcurrent != nil {
		params.McVal = sql.NullInt32{Int32: int32(*maxConcurrent), Valid: true}
		hasUpdate = true
	} else if clearMaxConcurrent {
		params.McClear = true
		hasUpdate = true
	}

	if hasUpdate {
		if err := q().UpdateEvent(ctx, params); err != nil {
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
	rules, err := q().GetEventScoringRules(ctx, eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if int(rules.ScoringType) != ScoringPoints {
		return nil
	}
	mode := ""
	if rules.ScoringRulesMode.Valid {
		mode = rules.ScoringRulesMode.String
	}
	var custom any
	if rules.CustomScoringTables.Valid && len(rules.CustomScoringTables.RawMessage) > 0 {
		var v any
		if err := json.Unmarshal(rules.CustomScoringTables.RawMessage, &v); err == nil {
			custom = v
		}
	}

	rows, err := q().ListStoredResultRows(ctx, eventID)
	if err != nil {
		return err
	}
	affected := map[string]bool{}
	var stale []storedResultRow
	for _, sr := range rows {
		r := storedResultRow{
			ID: sr.ID, UserID: sr.UserId,
			Position: sql.NullInt64{Int64: int64(sr.Position.Int32), Valid: sr.Position.Valid},
			Points:   int(sr.Points), ResultStatus: sr.ResultStatus, Grade: sr.Grade,
		}
		calculated := resolveResultPoints(mode, custom, &r)
		if r.Points != calculated {
			stale = append(stale, r)
		}
		affected[r.UserID] = true
	}
	for _, r := range stale {
		calculated := resolveResultPoints(mode, custom, &r)
		if err := q().UpdateRaceResultPoints(ctx, sqlc.UpdateRaceResultPointsParams{
			Points: int32(calculated), ID: r.ID,
		}); err != nil {
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
