package eventmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"encore.dev/beta/errs"
	"encore.app/auth"
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
	var exists bool
	if err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM "user" WHERE id=$1)`, userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return &errs.Error{Code: errs.NotFound, Message: "user not found"}
	}
	var granular sql.NullBool
	if err := db.QueryRow(ctx,
		`SELECT "granularParticipation" FROM "event" WHERE id=$1`, eventID).Scan(&granular); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &errs.Error{Code: errs.NotFound, Message: "event not found"}
		}
		return err
	}
	if granular.Valid && granular.Bool {
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM "race_event_member" WHERE "raceEventId"=$1 AND "userId"=$2)`,
			raceID, userID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return &errs.Error{Code: errs.FailedPrecondition, Message: "user is not registered for this race"}
		}
	} else {
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM "event_member" WHERE "eventId"=$1 AND "userId"=$2)`,
			eventID, userID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return &errs.Error{Code: errs.FailedPrecondition, Message: "user is not a member of this event"}
		}
	}
	return nil
}

// ListResultsCore loads the full ordered result list for a race.
func ListResultsCore(ctx context.Context, raceID string) ([]*RaceResultView, error) {
	rows, err := db.Query(ctx,
		`SELECT `+raceResultColumns+` FROM "race_result" WHERE "raceEventId"=$1
		 ORDER BY position ASC NULLS LAST, points DESC`, raceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*RaceResultView{}
	for rows.Next() {
		var r raceResultRow
		if err := rows.Scan(&r.ID, &r.RaceEventID, &r.UserID, &r.Position, &r.Points,
			&r.GateNumber, &r.FinishTime, &r.Margin, &r.PassingOrder, &r.Final3F,
			&r.ResultStatus, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, toRaceResultView(&r))
	}
	return out, rows.Err()
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
	var modeNS sql.NullString
	var customBytes []byte
	err = db.QueryRow(ctx,
		`SELECT "scoringType", "scoringRulesMode", "customScoringTables" FROM "event" WHERE id=$1`,
		eventID).Scan(&scoringType, &modeNS, &customBytes)
	if err != nil {
		return 0, "", nil, err
	}
	if modeNS.Valid {
		mode = modeNS.String
	}
	if len(customBytes) > 0 {
		var v any
		if uerr := json.Unmarshal(customBytes, &v); uerr == nil {
			custom = v
		}
	}
	return scoringType, mode, custom, nil
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
	var position any
	if entry.Position != nil {
		position = *entry.Position
	}
	var gateNumber any
	if entry.GateNumber != nil {
		gateNumber = int64(*entry.GateNumber)
	}
	var r raceResultRow
	err := db.QueryRow(ctx,
		`INSERT INTO "race_result" (id, "raceEventId", "userId", position, points,
		  "gateNumber", "finishTime", margin, "passingOrder", "final3F", "resultStatus",
		  "createdAt", "updatedAt")
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)
		 ON CONFLICT ("raceEventId", "userId") DO UPDATE SET
		  position = COALESCE($4, "race_result".position),
		  points = $5,
		  "gateNumber" = COALESCE($6, "race_result"."gateNumber"),
		  "finishTime" = COALESCE($7, "race_result"."finishTime"),
		  margin = COALESCE($8, "race_result".margin),
		  "passingOrder" = COALESCE($9, "race_result"."passingOrder"),
		  "final3F" = COALESCE($10, "race_result"."final3F"),
		  "resultStatus" = COALESCE($11, "race_result"."resultStatus"),
		  "updatedAt" = $12
		 RETURNING `+raceResultColumns,
		id, raceID, entry.UserID, position, points, gateNumber,
		entry.FinishTime, entry.Margin, entry.PassingOrder, entry.Final3F,
		entry.ResultStatus, now,
	).Scan(&r.ID, &r.RaceEventID, &r.UserID, &r.Position, &r.Points,
		&r.GateNumber, &r.FinishTime, &r.Margin, &r.PassingOrder, &r.Final3F,
		&r.ResultStatus, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	// Explicit clears (COALESCE keeps old values, so apply NULLs separately).
	if entry.ClearPosition || entry.ClearStatus {
		setClause := []string{}
		args := []any{}
		if entry.ClearPosition {
			setClause = append(setClause, "position = NULL")
		}
		if entry.ClearStatus {
			setClause = append(setClause, `"resultStatus" = NULL`)
		}
		_ = args
		if _, err := db.Exec(ctx,
			`UPDATE "race_result" SET `+strings.Join(setClause, ", ")+` WHERE id=$1`, r.ID); err != nil {
			return nil, err
		}
		if entry.ClearPosition {
			r.Position = sql.NullInt64{}
		}
		if entry.ClearStatus {
			r.ResultStatus = sql.NullString{}
		}
	}
	return &r, nil
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
	res, err := db.Exec(ctx,
		`DELETE FROM "race_result" WHERE "raceEventId"=$1 AND "userId"=$2`, p.RaceID, p.UserID)
	if err != nil {
		return nil, err
	}
	if res.RowsAffected() == 0 {
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
	rows, err := db.Query(ctx,
		`SELECT res.id, res."raceEventId", r.grade, res."userId", res.position,
		        res.points, res."resultStatus", u."classTier"
		 FROM "race_result" res
		 JOIN "race_event" r ON r.id = res."raceEventId"
		 JOIN "user" u ON u.id = res."userId"
		 WHERE r."eventId" = $1`, eventID)
	if err != nil {
		return err
	}
	var results []resultInfo
	for rows.Next() {
		var ri resultInfo
		if err := rows.Scan(&ri.id, &ri.raceID, &ri.grade, &ri.userID, &ri.position,
			&ri.points, &ri.resultStatus, &ri.classTier); err != nil {
			rows.Close()
			return err
		}
		results = append(results, ri)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
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
	grows, err := db.Query(ctx, `SELECT id, grade, sequence FROM "race_event" WHERE "eventId"=$1`, eventID)
	if err != nil {
		return err
	}
	for grows.Next() {
		var rid string
		var g sql.NullString
		var seq int
		if err := grows.Scan(&rid, &g, &seq); err != nil {
			grows.Close()
			return err
		}
		if g.Valid {
			raceGrades[rid] = g.String
		}
		raceSeqs[rid] = seq
	}
	grows.Close()
	if err := grows.Err(); err != nil {
		return err
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
	if err := db.QueryRow(ctx,
		`SELECT "granularParticipation" FROM "event" WHERE id=$1`, eventID).Scan(&granular); err != nil {
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
		mrows, err := db.Query(ctx, `SELECT "userId" FROM "event_member" WHERE "eventId"=$1`, eventID)
		if err != nil {
			return err
		}
		raceIDs, err := raceIDsForEvent(ctx, eventID)
		if err != nil {
			mrows.Close()
			return err
		}
		var memberIDs []string
		for mrows.Next() {
			var uid string
			if err := mrows.Scan(&uid); err != nil {
				mrows.Close()
				return err
			}
			memberIDs = append(memberIDs, uid)
		}
		mrows.Close()
		if err := mrows.Err(); err != nil {
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
	rmrows, err := db.Query(ctx,
		`SELECT m."raceEventId", m."userId" FROM "race_event_member" m
		 JOIN "race_event" r ON r.id = m."raceEventId" WHERE r."eventId"=$1`, eventID)
	if err != nil {
		return err
	}
	for rmrows.Next() {
		var rid, uid string
		if err := rmrows.Scan(&rid, &uid); err != nil {
			rmrows.Close()
			return err
		}
		addUser(rid, uid)
	}
	rmrows.Close()
	if err := rmrows.Err(); err != nil {
		return err
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
					if _, err := db.Exec(ctx,
						`INSERT INTO "race_result" (id, "raceEventId", "userId", points, "resultStatus", "createdAt", "updatedAt")
						 VALUES ($1,$2,$3,0,'DEFERRED',$4,$4)`,
						"raceresult-"+newID()[:8], raceID, userID, time.Now().UTC()); err != nil {
						return err
					}
				} else if !existing.resultStatus.Valid || existing.resultStatus.String == "DEFERRED" {
					if !existing.resultStatus.Valid || existing.points != 0 {
						if _, err := db.Exec(ctx,
							`UPDATE "race_result" SET "resultStatus"='DEFERRED', points=0 WHERE id=$1`, existing.id); err != nil {
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
					if _, err := db.Exec(ctx,
						`UPDATE "race_result" SET "resultStatus"=NULL, points=$1 WHERE id=$2`, restored, existing.id); err != nil {
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
	rows, err := db.Query(ctx, `SELECT id FROM "race_event" WHERE "eventId"=$1`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
