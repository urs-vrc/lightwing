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
)

// Per-race results. Event admins assign points to participants on a specific
// race event; the event-level aggregate is recomputed by scorecalc.
//
// Mirrors ts-legacy/eventmanager/results.ts.

// RaceResultView is the API view of one participant's result on a race.
type RaceResultView struct {
	ID           string  `json:"id"`
	RaceEventID  string  `json:"raceEventId"`
	UserID       string  `json:"userId"`
	Position     *int    `json:"position"`
	Points       int     `json:"points"`
	GateNumber   *int    `json:"gateNumber"`
	FinishTime   *string `json:"finishTime"`
	Margin       *string `json:"margin"`
	PassingOrder *string `json:"passingOrder"`
	Final3F      *string `json:"final3F"`
	ResultStatus *string `json:"resultStatus"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

type raceResultRow struct {
	ID           string
	RaceEventID  string
	UserID       string
	Position     sql.NullInt64
	Points       int
	GateNumber   sql.NullInt64
	FinishTime   sql.NullString
	Margin       sql.NullString
	PassingOrder sql.NullString
	Final3F      sql.NullString
	ResultStatus sql.NullString
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const raceResultColumns = `id, "raceEventId", "userId", position, points, "gateNumber", "finishTime", margin, "passingOrder", "final3F", "resultStatus", "createdAt", "updatedAt"`

func toRaceResultView(r *raceResultRow) *RaceResultView {
	return &RaceResultView{
		ID:           r.ID,
		RaceEventID:  r.RaceEventID,
		UserID:       r.UserID,
		Position:     nullInt(r.Position),
		Points:       r.Points,
		GateNumber:   nullInt(r.GateNumber),
		FinishTime:   nullString(r.FinishTime),
		Margin:       nullString(r.Margin),
		PassingOrder: nullString(r.PassingOrder),
		Final3F:      nullString(r.Final3F),
		ResultStatus: nullString(r.ResultStatus),
		CreatedAt:    isoTime(r.CreatedAt),
		UpdatedAt:    isoTime(r.UpdatedAt),
	}
}

// RequireRace asserts the race exists and belongs to the given event.
func RequireRace(ctx context.Context, eventID, raceID string) (*raceEventRow, error) {
	return requireRaceEvent(ctx, eventID, raceID)
}

// RequireMembershipForResult asserts the user exists and is registered based
// on the event's granularParticipation setting.
func RequireMembershipForResult(ctx context.Context, eventID, raceID, userID string) error {
	exists, err := q().UserExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return &errs.Error{Code: errs.NotFound, Message: "user not found"}
	}
	granular, err := q().GetEventGranularity(ctx, eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return &errs.Error{Code: errs.NotFound, Message: "event not found"}
	}
	if err != nil {
		return err
	}
	if granular {
		registered, err := q().RaceMemberExists(ctx, sqlc.RaceMemberExistsParams{
			RaceEventId: raceID,
			UserId:      userID,
		})
		if err != nil {
			return err
		}
		if !registered {
			return &errs.Error{Code: errs.FailedPrecondition, Message: "user is not registered for this race"}
		}
	} else {
		member, err := q().EventMemberExists(ctx, sqlc.EventMemberExistsParams{
			EventId: eventID,
			UserId:  userID,
		})
		if err != nil {
			return err
		}
		if !member {
			return &errs.Error{Code: errs.FailedPrecondition, Message: "user is not a member of this event"}
		}
	}
	return nil
}

// newRaceResultRow maps sqlc result columns onto the local raceResultRow.
func newRaceResultRow(id, raceEventID, userID string, position sql.NullInt32, points int32, gateNumber sql.NullInt16, finishTime, margin, passingOrder, final3F, resultStatus sql.NullString, createdAt, updatedAt time.Time) *raceResultRow {
	return &raceResultRow{
		ID: id, RaceEventID: raceEventID, UserID: userID,
		Position: sql.NullInt64{Int64: int64(position.Int32), Valid: position.Valid},
		Points:   int(points),
		GateNumber: sql.NullInt64{Int64: int64(gateNumber.Int16), Valid: gateNumber.Valid},
		FinishTime: finishTime, Margin: margin, PassingOrder: passingOrder,
		Final3F: final3F, ResultStatus: resultStatus,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
}

// ListResultsCore loads the full ordered result list for a race.
func ListResultsCore(ctx context.Context, raceID string) ([]*RaceResultView, error) {
	rows, err := q().ListRaceResults(ctx, raceID)
	if err != nil {
		return nil, err
	}
	out := []*RaceResultView{}
	for _, r := range rows {
		out = append(out, toRaceResultView(newRaceResultRow(
			r.ID, r.RaceEventId, r.UserId, r.Position, r.Points, r.GateNumber,
			r.FinishTime, r.Margin, r.PassingOrder, r.Final3F, r.ResultStatus,
			r.CreatedAt, r.UpdatedAt)))
	}
	return out, nil
}

// RaceResultInput mirrors the TS RaceResultInput: nil means omitted (leave
// unchanged on update), while non-nil values (including explicit zero/empty)
// overwrite. Explicit-null clearing is exposed via Clear* flags.
type RaceResultInput struct {
	UserID        string  `json:"userId"`
	Position      *int    `json:"position,omitempty"`
	Points        *int    `json:"points,omitempty"`
	GateNumber    *int    `json:"gateNumber,omitempty"`
	FinishTime    *string `json:"finishTime,omitempty"`
	Margin        *string `json:"margin,omitempty"`
	PassingOrder  *string `json:"passingOrder,omitempty"`
	Final3F       *string `json:"final3F,omitempty"`
	ResultStatus  *string `json:"resultStatus,omitempty"`
	ClearPosition bool    `json:"clearPosition,omitempty"`
	ClearStatus   bool    `json:"clearStatus,omitempty"`
}

// eventScoringRules loads an event's scoring mode + custom tables.
func eventScoringRules(ctx context.Context, eventID string) (scoringType int, mode string, custom any, err error) {
	rules, err := q().GetEventScoringRules(ctx, eventID)
	if err != nil {
		return 0, "", nil, err
	}
	if rules.ScoringRulesMode.Valid {
		mode = rules.ScoringRulesMode.String
	}
	if rules.CustomScoringTables.Valid && len(rules.CustomScoringTables.RawMessage) > 0 {
		var v any
		if uerr := json.Unmarshal(rules.CustomScoringTables.RawMessage, &v); uerr == nil {
			custom = v
		}
	}
	return int(rules.ScoringType), mode, custom, nil
}

// resolveEntryPoints computes the points to persist for a result entry on a
// points-based event; other scoring types keep the caller's points (default 0).
func resolveEntryPoints(scoringType int, mode string, custom any, grade string, entry *RaceResultInput) int {
	points := 0
	if entry.Points != nil {
		points = *entry.Points
	}
	if scoringType == ScoringPoints {
		status := ""
		if entry.ResultStatus != nil {
			status = *entry.ResultStatus
		}
		points = ResolvePoints(mode, custom, grade, entry.Position, status)
	}
	return points
}

// upsertRaceResult inserts or updates a participant's result. Create sets all
// fields (absent numerics default, absent text is NULL); update only touches
// fields present in the entry, mirroring buildRaceResultUpsert.
func upsertRaceResult(ctx context.Context, raceID string, entry *RaceResultInput, points int) (*raceResultRow, error) {
	id := "raceresult-" + newID()[:8]
	now := time.Now().UTC()
	var position sql.NullInt32
	if entry.Position != nil {
		position = sql.NullInt32{Int32: int32(*entry.Position), Valid: true}
	}
	var gateNumber sql.NullInt16
	if entry.GateNumber != nil {
		gateNumber = sql.NullInt16{Int16: int16(*entry.GateNumber), Valid: true}
	}
	upserted, err := q().UpsertRaceResult(ctx, sqlc.UpsertRaceResultParams{
		ID: id, RaceEventId: raceID, UserId: entry.UserID, Position: position,
		Points: int32(points), GateNumber: gateNumber,
		FinishTime: nullStringFromPtr(entry.FinishTime),
		Margin:     nullStringFromPtr(entry.Margin),
		PassingOrder: nullStringFromPtr(entry.PassingOrder),
		Final3F:      nullStringFromPtr(entry.Final3F),
		ResultStatus: nullStringFromPtr(entry.ResultStatus),
		CreatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	r := newRaceResultRow(
		upserted.ID, upserted.RaceEventId, upserted.UserId, upserted.Position,
		upserted.Points, upserted.GateNumber, upserted.FinishTime, upserted.Margin,
		upserted.PassingOrder, upserted.Final3F, upserted.ResultStatus,
		upserted.CreatedAt, upserted.UpdatedAt)
	// Explicit clears (COALESCE keeps old values, so apply NULLs separately).
	if entry.ClearPosition && entry.ClearStatus {
		if err := q().ClearRaceResultPositionAndStatus(ctx, r.ID); err != nil {
			return nil, err
		}
		r.Position = sql.NullInt64{}
		r.ResultStatus = sql.NullString{}
	} else if entry.ClearPosition {
		if err := q().ClearRaceResultPosition(ctx, r.ID); err != nil {
			return nil, err
		}
		r.Position = sql.NullInt64{}
	} else if entry.ClearStatus {
		if err := q().ClearRaceResultStatus(ctx, r.ID); err != nil {
			return nil, err
		}
		r.ResultStatus = sql.NullString{}
	}
	return r, nil
}

// submitCalcForUsers queues a scorecalc job, skipping empty user lists
// (SubmitCalc rejects them).
func submitCalcForUsers(ctx context.Context, eventID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	_, err := scorecalc.SubmitCalc(ctx, &scorecalc.SubmitCalcParams{EventID: eventID, UserIDs: userIDs})
	return err
}

// --- Assign ---

// AssignRaceEventRequest mirrors AssignRaceResultParams. Result fields are
// explicit (rather than an embedded struct) so they appear in the generated
// API client schema.
type AssignRaceResultRequest struct {
	EventID       string  `json:"eventId"`
	RaceID        string  `json:"raceId"`
	UserID        string  `json:"userId"`
	Authorization string  `header:"Authorization"`
	Position      *int    `json:"position,omitempty"`
	Points        *int    `json:"points,omitempty"`
	GateNumber    *int    `json:"gateNumber,omitempty"`
	FinishTime    *string `json:"finishTime,omitempty"`
	Margin        *string `json:"margin,omitempty"`
	PassingOrder  *string `json:"passingOrder,omitempty"`
	Final3F       *string `json:"final3F,omitempty"`
	ResultStatus  *string `json:"resultStatus,omitempty"`
	ClearPosition bool    `json:"clearPosition,omitempty"`
	ClearStatus   bool    `json:"clearStatus,omitempty"`
}

// toResultInput maps the request's result fields onto a RaceResultInput.
func (p *AssignRaceResultRequest) toResultInput() RaceResultInput {
	return RaceResultInput{
		UserID:        p.UserID,
		Position:      p.Position,
		Points:        p.Points,
		GateNumber:    p.GateNumber,
		FinishTime:    p.FinishTime,
		Margin:        p.Margin,
		PassingOrder:  p.PassingOrder,
		Final3F:       p.Final3F,
		ResultStatus:  p.ResultStatus,
		ClearPosition: p.ClearPosition,
		ClearStatus:   p.ClearStatus,
	}
}

// AssignRaceResultCore assigns (or updates) a participant's result on a race.
func AssignRaceResultCore(ctx context.Context, p *AssignRaceResultRequest) (*RaceResultView, error) {
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
	if err := RequireMembershipForResult(ctx, p.EventID, p.RaceID, p.UserID); err != nil {
		return nil, err
	}
	// The path-level userId is the result owner, mirroring TS where the whole
	// params object (including userId) is passed as the upsert entry.
	entry := p.toResultInput()
	mode := ""
	if e.ScoringRulesMode.Valid {
		mode = e.ScoringRulesMode.String
	}
	var custom any
	if len(e.CustomScoringTables) > 0 {
		var v any
		if uerr := json.Unmarshal(e.CustomScoringTables, &v); uerr == nil {
			custom = v
		}
	}
	grade := ""
	if race.Grade.Valid {
		grade = race.Grade.String
	}
	points := resolveEntryPoints(e.ScoringType, mode, custom, grade, &entry)
	result, err := upsertRaceResult(ctx, p.RaceID, &entry, points)
	if err != nil {
		return nil, err
	}
	if err := ApplyAutoDeferralsForEvent(ctx, p.EventID, &p.RaceID); err != nil {
		return nil, err
	}
	if err := submitCalcForUsers(ctx, p.EventID, []string{p.UserID}); err != nil {
		return nil, err
	}
	return toRaceResultView(result), nil
}

//encore:api public method=PUT path=/api/race-results
func AssignRaceResult(ctx context.Context, p *AssignRaceResultRequest) (*RaceResultView, error) {
	return AssignRaceResultCore(ctx, p)
}

// --- Delete ---

// DeleteRaceResultRequest mirrors DeleteRaceResultParams.
type DeleteRaceResultRequest struct {
	EventID       string `json:"eventId"`
	RaceID        string `json:"raceId"`
	UserID        string `json:"userId"`
	Authorization string `header:"Authorization"`
}

// DeleteRaceResultResponse reports deletion.
type DeleteRaceResultResponse struct {
	Deleted bool `json:"deleted"`
}

// DeleteRaceResultCore removes a participant's result from a race.
func DeleteRaceResultCore(ctx context.Context, p *DeleteRaceResultRequest) (*DeleteRaceResultResponse, error) {
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	if _, err := RequireRace(ctx, p.EventID, p.RaceID); err != nil {
		return nil, err
	}
	affected, err := q().DeleteRaceResult(ctx, sqlc.DeleteRaceResultParams{
		RaceEventId: p.RaceID,
		UserId:      p.UserID,
	})
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, &errs.Error{Code: errs.NotFound, Message: "result not found"}
	}
	if err := ApplyAutoDeferralsForEvent(ctx, p.EventID, &p.RaceID); err != nil {
		return nil, err
	}
	if err := submitCalcForUsers(ctx, p.EventID, []string{p.UserID}); err != nil {
		return nil, err
	}
	return &DeleteRaceResultResponse{Deleted: true}, nil
}

//encore:api public method=DELETE path=/api/race-results
func DeleteRaceResult(ctx context.Context, p *DeleteRaceResultRequest) (*DeleteRaceResultResponse, error) {
	return DeleteRaceResultCore(ctx, p)
}

// --- List ---

// RaceResultsQuery lists results of a race.
type RaceResultsQuery struct {
	EventID string `query:"eventId"`
	RaceID  string `query:"raceId"`
}

// RaceResultsResponse wraps the ordered result list.
type RaceResultsResponse struct {
	Results []*RaceResultView `json:"results"`
}

// ListRaceResultsCore lists results recorded for a single race.
func ListRaceResultsCore(ctx context.Context, q *RaceResultsQuery) (*RaceResultsResponse, error) {
	if _, err := RequireRace(ctx, q.EventID, q.RaceID); err != nil {
		return nil, err
	}
	results, err := ListResultsCore(ctx, q.RaceID)
	if err != nil {
		return nil, err
	}
	return &RaceResultsResponse{Results: results}, nil
}

//encore:api public method=GET path=/api/race-results-list
func ListRaceResults(ctx context.Context, q *RaceResultsQuery) (*RaceResultsResponse, error) {
	return ListRaceResultsCore(ctx, q)
}

// --- Auto-deferral ---

// Auto-deferral: a user placing 1st in an auto-defer grade race (e.g. OP)
// without a grade yet (classTier null/PRE_OP/OP) who is signed up for
// multiple races gets DEFERRED in their other races. When they no longer
// place 1st anywhere auto-defer, their DEFERRED rows revert (except the
// race currently being modified).
//
// Mirrors applyAutoDeferralsForEvent in ts-legacy/eventmanager/results.ts.
func ApplyAutoDeferralsForEvent(ctx context.Context, eventID string, modifiedRaceID *string) error {
	scoringType, mode, custom, err := eventScoringRules(ctx, eventID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if scoringType != ScoringPoints {
		return nil
	}
	type resultInfo struct {
		id           string
		raceID       string
		grade        sql.NullString
		userID       string
		position     sql.NullInt64
		points       int
		resultStatus sql.NullString
		classTier    sql.NullString
	}
	rows, err := q().ListAutoDeferralInputs(ctx, eventID)
	if err != nil {
		return err
	}
	var results []resultInfo
	for _, r := range rows {
		results = append(results, resultInfo{
			id: r.ID, raceID: r.RaceEventId, grade: r.Grade, userID: r.UserId,
			position: sql.NullInt64{Int64: int64(r.Position.Int32), Valid: r.Position.Valid},
			points:   int(r.Points), resultStatus: r.ResultStatus,
			classTier: nullStringFromAny(r.ClassTier),
		})
	}
	isUngraded := func(tier sql.NullString) bool {
		return !tier.Valid || tier.String == "PRE_OP" || tier.String == "OP"
	}
	gradeStr := func(g sql.NullString) string {
		if g.Valid {
			return g.String
		}
		return ""
	}
	isTerminal := func(status sql.NullString) bool {
		return status.Valid && (status.String == "DSQ" || status.String == "DNF" || status.String == "DNS")
	}
	raceGrades := map[string]string{}
	raceSeqs := map[string]int{}
	grows, err := q().ListEligibleRaces(ctx, eventID)
	if err != nil {
		return err
	}
	for _, gr := range grows {
		if gr.Grade.Valid {
			raceGrades[gr.ID] = gr.Grade.String
		}
		raceSeqs[gr.ID] = int(gr.Sequence)
	}

	// Qualification: 1st place in an auto-defer grade, ungraded, no terminal status.
	// Find earliest (lowest race sequence) win for each user.
	earliestWinSeq := map[string]int{}
	for _, res := range results {
		if !res.position.Valid || res.position.Int64 != 1 || isTerminal(res.resultStatus) {
			continue
		}
		if !IsAutoDeferGrade(mode, custom, gradeStr(res.grade)) {
			continue
		}
		if isUngraded(res.classTier) {
			seq := raceSeqs[res.raceID]
			if minSeq, ok := earliestWinSeq[res.userID]; !ok || seq < minSeq {
				earliestWinSeq[res.userID] = seq
			}
		}
	}
	// All registered users per race: event members (non-granular) + result
	// holders + explicit race members.
	var granular bool
	granular, err = q().GetEventGranularity(ctx, eventID)
	if err != nil {
		return err
	}
	raceUsers := map[string]map[string]bool{}
	addUser := func(raceID, userID string) {
		if raceUsers[raceID] == nil {
			raceUsers[raceID] = map[string]bool{}
		}
		raceUsers[raceID][userID] = true
	}
	if !granular {
		memberIDs, err := q().ListEventMemberIDs(ctx, eventID)
		if err != nil {
			return err
		}
		raceIDs, err := raceIDsForEvent(ctx, eventID)
		if err != nil {
			return err
		}
		for _, rid := range raceIDs {
			for _, uid := range memberIDs {
				addUser(rid, uid)
			}
		}
	}
	for _, res := range results {
		addUser(res.raceID, res.userID)
	}
	rmrows, err := q().ListRaceMemberPairs(ctx, eventID)
	if err != nil {
		return err
	}
	for _, rm := range rmrows {
		addUser(rm.RaceEventId, rm.UserId)
	}
	lookup := map[string]map[string]*resultInfo{}
	for i := range results {
		ri := &results[i]
		if lookup[ri.raceID] == nil {
			lookup[ri.raceID] = map[string]*resultInfo{}
		}
		lookup[ri.raceID][ri.userID] = ri
	}
	for raceID, users := range raceUsers {
		seq := raceSeqs[raceID]
		for userID := range users {
			existing := lookup[raceID][userID]
			winSeq, hasWin := earliestWinSeq[userID]
			shouldDefer := hasWin && seq > winSeq

			if shouldDefer {
				if existing == nil {
					if err := q().InsertDeferredResult(ctx, sqlc.InsertDeferredResultParams{
						ID: "raceresult-" + newID()[:8], RaceEventId: raceID, UserId: userID,
						CreatedAt: time.Now().UTC(),
					}); err != nil {
						return err
					}
				} else if !existing.resultStatus.Valid || existing.resultStatus.String == "DEFERRED" {
					if !existing.resultStatus.Valid || existing.points != 0 {
						if err := q().MarkResultDeferred(ctx, existing.id); err != nil {
							return err
						}
					}
				}
			} else {
				if modifiedRaceID != nil && raceID == *modifiedRaceID {
					continue
				}
				if existing != nil && existing.resultStatus.Valid && existing.resultStatus.String == "DEFERRED" {
					var pos *int
					if existing.position.Valid {
						n := int(existing.position.Int64)
						pos = &n
					}
					restored := ResolvePoints(mode, custom, raceGrades[raceID], pos, "")
					if err := q().RestoreDeferredResult(ctx, sqlc.RestoreDeferredResultParams{
						Points: int32(restored), ID: existing.id,
					}); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// raceIDsForEvent returns all race ids of an event.
func raceIDsForEvent(ctx context.Context, eventID string) ([]string, error) {
	return q().ListRaceIDs(ctx, eventID)
}
