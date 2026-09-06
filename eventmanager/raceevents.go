package eventmanager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"encore.dev/beta/errs"
	"encore.app/auth"
)

// Race events: single races contained within an event. The parent event acts
// as the container/scoreboard; each race carries its own distance, track
// type and location. Every mutation is gated by event permission on the
// parent, so organization-owned and user-owned events (and site admins)
// behave uniformly.
//
// Mirrors ts-legacy/eventmanager/raceevents.ts. Routes are flat top-level
// paths with ids in the body (per repo convention — the Encore Go parser
// rejects struct params combined with :path params).

// RaceEventDetail is the full race view returned by race endpoints, mirroring
// ts-legacy/eventmanager/raceevents.ts RaceEventDetail. Members reuse the
// shared RaceEventMemberView from events.go.
type RaceEventDetail struct {
	ID                string                `json:"id"`
	EventID           string                `json:"eventId"`
	Name              string                `json:"name"`
	Sequence          int                   `json:"sequence"`
	DistanceMeters    int                   `json:"distanceMeters"`
	TrackType         string                `json:"trackType"`
	Location          string                `json:"location"`
	ScoringType       *int                  `json:"scoringType"`
	Grade             *string               `json:"grade"`
	ClassRestriction  *string               `json:"classRestriction"`
	StartsAt          *string               `json:"startsAt"`
	EndsAt            *string               `json:"endsAt"`
	ParticipantLimit  *int                  `json:"participantLimit"`
	CreatedAt         string                `json:"createdAt"`
	UpdatedAt         string                `json:"updatedAt"`
	Members           []RaceEventMemberView `json:"members"`
}

type raceEventRow struct {
	ID               string
	EventID          string
	Name             string
	Sequence         int
	DistanceMeters   int
	TrackType        string
	Location         string
	ScoringType      sql.NullInt64
	Grade            sql.NullString
	ClassRestriction sql.NullString
	StartsAt         *time.Time
	EndsAt           *time.Time
	ParticipantLimit sql.NullInt64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

const raceEventColumns = `id, "eventId", name, sequence, "distanceMeters", "trackType", location, "scoringType", grade, "classRestriction", "startsAt", "endsAt", "participantLimit", "createdAt", "updatedAt"`

// loadRaceMembers returns the registered participants of a race with display
// names (vrchatUsername preferred) and class tiers.
func loadRaceMembers(ctx context.Context, raceID string) ([]RaceEventMemberView, error) {
	rows, err := db.Query(ctx,
		`SELECT m."userId", COALESCE(u."vrchatUsername", u.name), u."classTier"
		 FROM "race_event_member" m JOIN "user" u ON u.id = m."userId"
		 WHERE m."raceEventId" = $1`, raceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []RaceEventMemberView{}
	for rows.Next() {
		var m RaceEventMemberView
		var tier sql.NullString
		if err := rows.Scan(&m.UserID, &m.Name, &tier); err != nil {
			return nil, err
		}
		m.ClassTier = nullString(tier)
		members = append(members, m)
	}
	return members, rows.Err()
}

func loadRaceMembersForEvent(ctx context.Context, eventID string) (map[string][]RaceEventMemberView, error) {
	rows, err := db.Query(ctx,
		`SELECT m."raceEventId", m."userId", COALESCE(u."vrchatUsername", u.name), u."classTier"
		 FROM "race_event_member" m
		 JOIN "race_event" r ON r.id = m."raceEventId"
		 JOIN "user" u ON u.id = m."userId"
		 WHERE r."eventId" = $1`, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to load race members for event: %w", err)
	}
	defer rows.Close()

	membersByRace := make(map[string][]RaceEventMemberView)
	for rows.Next() {
		var raceID string
		var m RaceEventMemberView
		var tier sql.NullString
		if err := rows.Scan(&raceID, &m.UserID, &m.Name, &tier); err != nil {
			return nil, fmt.Errorf("failed to scan race member: %w", err)
		}
		m.ClassTier = classTierPtr(tier)
		membersByRace[raceID] = append(membersByRace[raceID], m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate race members: %w", err)
	}
	return membersByRace, nil
}

func toRaceEventDetail(r *raceEventRow, members []RaceEventMemberView) *RaceEventDetail {
	if members == nil {
		members = []RaceEventMemberView{}
	}
	return &RaceEventDetail{
		ID:               r.ID,
		EventID:          r.EventID,
		Name:             r.Name,
		Sequence:         r.Sequence,
		DistanceMeters:   r.DistanceMeters,
		TrackType:        r.TrackType,
		Location:         r.Location,
		ScoringType:      nullInt(r.ScoringType),
		Grade:            nullString(r.Grade),
		ClassRestriction: classTierPtr(r.ClassRestriction),
		StartsAt:         nullTime(r.StartsAt),
		EndsAt:           nullTime(r.EndsAt),
		ParticipantLimit: nullInt(r.ParticipantLimit),
		CreatedAt:        isoTime(r.CreatedAt),
		UpdatedAt:        isoTime(r.UpdatedAt),
		Members:          members,
	}
}

// requireRaceEvent loads a race and asserts it belongs to the given event.
func requireRaceEvent(ctx context.Context, eventID, raceID string) (*raceEventRow, error) {
	var r raceEventRow
	err := db.QueryRow(ctx,
		`SELECT `+raceEventColumns+` FROM "race_event" WHERE id = $1`, raceID,
	).Scan(&r.ID, &r.EventID, &r.Name, &r.Sequence, &r.DistanceMeters,
		&r.TrackType, &r.Location, &r.ScoringType, &r.Grade, &r.ClassRestriction,
		&r.StartsAt, &r.EndsAt, &r.ParticipantLimit, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) || r.EventID != eventID {
		return nil, &errs.Error{Code: errs.NotFound, Message: "race not found"}
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// loadRaceDetail returns the full detail view for a race row.
func loadRaceDetail(ctx context.Context, r *raceEventRow) (*RaceEventDetail, error) {
	members, err := loadRaceMembers(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	return toRaceEventDetail(r, members), nil
}

// requireEventRow loads the parent event row or returns NotFound.
func requireEventRow(ctx context.Context, eventID string) (*eventRow, error) {
	e, err := scanEventRow(db.QueryRow(ctx,
		`SELECT `+eventColumns+` FROM "event" WHERE id = $1`, eventID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "event not found"}
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

// parseTimePtr parses an optional ISO timestamp into UTC, or nil when empty.
func parseTimePtr(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: fmt.Sprintf("invalid timestamp: %s", *s)}
	}
	utc := t.UTC()
	return &utc, nil
}

// --- Create ---

// CreateRaceEventRequest mirrors CreateRaceEventParams (POST /api/events/:eventId/races).
type CreateRaceEventRequest struct {
	EventID           string  `json:"eventId"`
	Authorization     string  `header:"Authorization"`
	Name              string  `json:"name"`
	Sequence          *int    `json:"sequence,omitempty"`
	DistanceMeters    int     `json:"distanceMeters"`
	TrackType         string  `json:"trackType"`
	Location          string  `json:"location"`
	ScoringType       *int    `json:"scoringType,omitempty"`
	Grade             *string `json:"grade,omitempty"`
	ClassRestriction  *string `json:"classRestriction,omitempty"`
	StartsAt          *string `json:"startsAt,omitempty"`
	EndsAt            *string `json:"endsAt,omitempty"`
	ParticipantLimit  OptInt  `json:"participantLimit,omitempty"`
}

// CreateRaceEventCore adds a race to an event, gated by event-create permission.
func CreateRaceEventCore(ctx context.Context, p *CreateRaceEventRequest) (*RaceEventDetail, error) {
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionCreate); err != nil {
		return nil, err
	}
	e, err := requireEventRow(ctx, p.EventID)
	if err != nil {
		return nil, err
	}
	limit := p.ParticipantLimit.Value
	if p.ParticipantLimit.Set && limit != nil && !e.GranularParticipation {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "Race participant limit can only be configured for granular events"}
	}
	var maxSeq sql.NullInt64
	if err := db.QueryRow(ctx,
		`SELECT MAX(sequence) FROM "race_event" WHERE "eventId" = $1`, p.EventID,
	).Scan(&maxSeq); err != nil {
		return nil, err
	}
	seq := 1
	if maxSeq.Valid {
		seq = int(maxSeq.Int64) + 1
	}
	if p.Sequence != nil {
		seq = *p.Sequence
	}
	startsAt, err := parseTimePtr(p.StartsAt)
	if err != nil {
		return nil, err
	}
	endsAt, err := parseTimePtr(p.EndsAt)
	if err != nil {
		return nil, err
	}
	var scoringType any
	if p.ScoringType != nil {
		scoringType = *p.ScoringType
	}
	id := "race-" + newID()[:8]
	now := time.Now().UTC()
	_, err = db.Exec(ctx,
		`INSERT INTO "race_event" (id, "eventId", name, sequence, "distanceMeters", "trackType",
		  location, "scoringType", grade, "classRestriction", "startsAt", "endsAt",
		  "participantLimit", "createdAt", "updatedAt")
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$14)`,
		id, p.EventID, p.Name, seq, p.DistanceMeters, p.TrackType, p.Location,
		scoringType, p.Grade, p.ClassRestriction, startsAt, endsAt, limit, now)
	if err != nil {
		return nil, err
	}
	r, err := requireRaceEvent(ctx, p.EventID, id)
	if err != nil {
		return nil, err
	}
	return loadRaceDetail(ctx, r)
}

//encore:api public method=POST path=/api/race-events
func CreateRaceEvent(ctx context.Context, p *CreateRaceEventRequest) (*RaceEventDetail, error) {
	return CreateRaceEventCore(ctx, p)
}

// --- Reorder ---

// ReorderRaceEventsRequest mirrors ReorderRaceEventsParams.
type ReorderRaceEventsRequest struct {
	EventID        string   `json:"eventId"`
	Authorization  string   `header:"Authorization"`
	OrderedRaceIDs []string `json:"orderedRaceIds"`
}

// ReorderRaceEventsResponse returns the reordered race list.
type ReorderRaceEventsResponse struct {
	Races []*RaceEventDetail `json:"races"`
}

// ReorderRaceEventsCore reorders races, requiring full coverage and no duplicates.
func ReorderRaceEventsCore(ctx context.Context, p *ReorderRaceEventsRequest) (*ReorderRaceEventsResponse, error) {
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	rows, err := db.Query(ctx,
		`SELECT id, sequence FROM "race_event" WHERE "eventId" = $1 ORDER BY sequence ASC`, p.EventID)
	if err != nil {
		return nil, err
	}
	var ids []string
	var seqs []int
	for rows.Next() {
		var id string
		var seq int
		if err := rows.Scan(&id, &seq); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
		seqs = append(seqs, seq)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	existing := map[string]bool{}
	for _, id := range ids {
		existing[id] = true
	}
	seen := map[string]bool{}
	for _, id := range p.OrderedRaceIDs {
		if seen[id] {
			return nil, &errs.Error{Code: errs.InvalidArgument, Message: "duplicate race IDs are not allowed in the payload"}
		}
		seen[id] = true
	}
	if len(p.OrderedRaceIDs) != len(ids) {
		return nil, &errs.Error{Code: errs.InvalidArgument,
			Message: fmt.Sprintf("payload must contain exactly all %d race IDs associated with this event", len(ids))}
	}
	for _, id := range p.OrderedRaceIDs {
		if !existing[id] {
			return nil, &errs.Error{Code: errs.InvalidArgument,
				Message: fmt.Sprintf("race ID %q does not belong to event %q", id, p.EventID)}
		}
	}
	needsUpdate := false
	for i := range ids {
		if ids[i] != p.OrderedRaceIDs[i] || seqs[i] != i+1 {
			needsUpdate = true
			break
		}
	}
	if needsUpdate {
		tx, err := db.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()
		for i, id := range p.OrderedRaceIDs {
			if _, err := tx.Exec(ctx, `UPDATE "race_event" SET sequence = $1 WHERE id = $2`, i+1, id); err != nil {
				return nil, err
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}
	return ListRaceEventsCore(ctx, &ListRaceEventsQuery{EventID: p.EventID})
}

//encore:api public method=PUT path=/api/race-events-reorder
func ReorderRaceEvents(ctx context.Context, p *ReorderRaceEventsRequest) (*ReorderRaceEventsResponse, error) {
	return ReorderRaceEventsCore(ctx, p)
}

// --- List / get ---

// ListRaceEventsQuery lists races of an event ordered by sequence.
type ListRaceEventsQuery struct {
	EventID string `query:"eventId"`
}

// ListRaceEventsCore returns all races of an event with members.
func ListRaceEventsCore(ctx context.Context, q *ListRaceEventsQuery) (*ReorderRaceEventsResponse, error) {
	rows, err := db.Query(ctx,
		`SELECT `+raceEventColumns+` FROM "race_event" WHERE "eventId" = $1 ORDER BY sequence ASC`, q.EventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var raceRows []raceEventRow
	for rows.Next() {
		var r raceEventRow
		if err := rows.Scan(&r.ID, &r.EventID, &r.Name, &r.Sequence, &r.DistanceMeters,
			&r.TrackType, &r.Location, &r.ScoringType, &r.Grade, &r.ClassRestriction,
			&r.StartsAt, &r.EndsAt, &r.ParticipantLimit, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		raceRows = append(raceRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	membersByRace, err := loadRaceMembersForEvent(ctx, q.EventID)
	if err != nil {
		return nil, err
	}

	resp := &ReorderRaceEventsResponse{Races: make([]*RaceEventDetail, 0, len(raceRows))}
	for _, r := range raceRows {
		mems := membersByRace[r.ID]
		resp.Races = append(resp.Races, toRaceEventDetail(&r, mems))
	}
	return resp, nil
}

//encore:api public method=GET path=/api/race-events-list
func ListRaceEvents(ctx context.Context, q *ListRaceEventsQuery) (*ReorderRaceEventsResponse, error) {
	return ListRaceEventsCore(ctx, q)
}

// RaceEventQuery fetches a single race.
type RaceEventQuery struct {
	EventID string `query:"eventId"`
	RaceID  string `query:"raceId"`
}

//encore:api public method=GET path=/api/race-event
func GetRaceEvent(ctx context.Context, q *RaceEventQuery) (*RaceEventDetail, error) {
	return GetRaceEventCore(ctx, q)
}

// GetRaceEventCore returns a single race within an event.
func GetRaceEventCore(ctx context.Context, q *RaceEventQuery) (*RaceEventDetail, error) {
	r, err := requireRaceEvent(ctx, q.EventID, q.RaceID)
	if err != nil {
		return nil, err
	}
	return loadRaceDetail(ctx, r)
}

// --- Update ---

// UpdateRaceEventRequest mirrors UpdateRaceEventParams. Pointer fields
// distinguish omitted (nil, leave unchanged) from explicit null (clear).
type UpdateRaceEventRequest struct {
	EventID           string   `json:"eventId"`
	RaceID            string   `json:"raceId"`
	Authorization     string   `header:"Authorization"`
	Name              *string  `json:"name,omitempty"`
	Sequence          *int     `json:"sequence,omitempty"`
	DistanceMeters    *int     `json:"distanceMeters,omitempty"`
	TrackType         *string  `json:"trackType,omitempty"`
	Location          *string  `json:"location,omitempty"`
	ScoringType       *int     `json:"scoringType,omitempty"`
	ClearScoringType  bool     `json:"clearScoringType,omitempty"`
	Grade             *string  `json:"grade,omitempty"`
	ClearGrade        bool     `json:"clearGrade,omitempty"`
	ClassRestriction  *string  `json:"classRestriction,omitempty"`
	ClearClassRestr   bool     `json:"clearClassRestriction,omitempty"`
	StartsAt          *string  `json:"startsAt,omitempty"`
	ClearStartsAt     bool     `json:"clearStartsAt,omitempty"`
	EndsAt            *string  `json:"endsAt,omitempty"`
	ClearEndsAt       bool     `json:"clearEndsAt,omitempty"`
	ParticipantLimit  OptInt   `json:"participantLimit,omitempty"`
}

// UpdateRaceEventCore updates editable race fields. A grade change triggers
// points recomputation for the event.
func UpdateRaceEventCore(ctx context.Context, p *UpdateRaceEventRequest) (*RaceEventDetail, error) {
	existing, err := requireRaceEvent(ctx, p.EventID, p.RaceID)
	if err != nil {
		return nil, err
	}
	e, err := requireEventRow(ctx, p.EventID)
	if err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	var limit any
	if existing.ParticipantLimit.Valid {
		limit = int(existing.ParticipantLimit.Int64)
	}
	limitVal := p.ParticipantLimit.Value
	if p.ParticipantLimit.Set {
		if limitVal != nil && !e.GranularParticipation {
			return nil, &errs.Error{Code: errs.InvalidArgument, Message: "Race participant limit can only be configured for granular events"}
		}
		if limitVal != nil {
			var currentCount int
			if err := db.QueryRow(ctx,
				`SELECT COUNT(*) FROM "race_event_member" WHERE "raceEventId" = $1`, p.RaceID,
			).Scan(&currentCount); err != nil {
				return nil, err
			}
			if err := AssertLimitCanBeReduced(currentCount, *limitVal,
				CodeParticipantLimitBelowEnrollment,
				"Participant limit cannot be lower than the current enrollment"); err != nil {
				return nil, err
			}
			limit = *limitVal
		} else {
			limit = nil
		}
	}
	triggerRecomputation := false
	if p.Grade != nil {
		var oldGrade string
		if existing.Grade.Valid {
			oldGrade = existing.Grade.String
		}
		if *p.Grade != oldGrade {
			triggerRecomputation = true
		}
	}
	startsAt := existing.StartsAt
	if p.ClearStartsAt {
		startsAt = nil
	} else if p.StartsAt != nil {
		parsed, perr := parseTimePtr(p.StartsAt)
		if perr != nil {
			return nil, perr
		}
		startsAt = parsed
	}
	endsAt := existing.EndsAt
	if p.ClearEndsAt {
		endsAt = nil
	} else if p.EndsAt != nil {
		endsAt, err = parseTimePtr(p.EndsAt)
		if err != nil {
			return nil, err
		}
	}
	name := existing.Name
	if p.Name != nil {
		name = *p.Name
	}
	seq := existing.Sequence
	if p.Sequence != nil {
		seq = *p.Sequence
	}
	distance := existing.DistanceMeters
	if p.DistanceMeters != nil {
		distance = *p.DistanceMeters
	}
	trackType := existing.TrackType
	if p.TrackType != nil {
		trackType = *p.TrackType
	}
	location := existing.Location
	if p.Location != nil {
		location = *p.Location
	}
	var scoringType any
	if existing.ScoringType.Valid {
		scoringType = int(existing.ScoringType.Int64)
	}
	if p.ClearScoringType {
		scoringType = nil
	} else if p.ScoringType != nil {
		scoringType = *p.ScoringType
	}
	var grade any
	if existing.Grade.Valid {
		grade = existing.Grade.String
	}
	if p.ClearGrade {
		grade = nil
	} else if p.Grade != nil {
		grade = *p.Grade
	}
	var classRestr any
	if existing.ClassRestriction.Valid {
		classRestr = existing.ClassRestriction.String
	}
	if p.ClearClassRestr {
		classRestr = nil
	} else if p.ClassRestriction != nil {
		classRestr = *p.ClassRestriction
	}
	now := time.Now().UTC()
	_, err = db.Exec(ctx,
		`UPDATE "race_event" SET name=$1, sequence=$2, "distanceMeters"=$3, "trackType"=$4,
		  location=$5, "scoringType"=$6, grade=$7, "classRestriction"=$8, "startsAt"=$9,
		  "endsAt"=$10, "participantLimit"=$11, "updatedAt"=$12 WHERE id=$13`,
		name, seq, distance, trackType, location, scoringType, grade, classRestr,
		startsAt, endsAt, limit, now, p.RaceID)
	if err != nil {
		return nil, err
	}
	if triggerRecomputation {
		if err := recomputeEventPoints(ctx, p.EventID); err != nil {
			return nil, err
		}
	}
	updated, err := requireRaceEvent(ctx, p.EventID, p.RaceID)
	if err != nil {
		return nil, err
	}
	return loadRaceDetail(ctx, updated)
}

//encore:api public method=PATCH path=/api/race-events
func UpdateRaceEvent(ctx context.Context, p *UpdateRaceEventRequest) (*RaceEventDetail, error) {
	return UpdateRaceEventCore(ctx, p)
}

// --- Delete ---

// DeleteRaceEventRequest mirrors DeleteRaceEventParams.
type DeleteRaceEventRequest struct {
	EventID       string `json:"eventId"`
	RaceID        string `json:"raceId"`
	Authorization string `header:"Authorization"`
}

// DeleteRaceEventResponse reports deletion.
type DeleteRaceEventResponse struct {
	Deleted bool `json:"deleted"`
}

// DeleteRaceEventCore deletes a race and its results (cascade).
func DeleteRaceEventCore(ctx context.Context, p *DeleteRaceEventRequest) (*DeleteRaceEventResponse, error) {
	if _, err := requireRaceEvent(ctx, p.EventID, p.RaceID); err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionDelete); err != nil {
		return nil, err
	}
	if _, err := db.Exec(ctx, `DELETE FROM "race_event" WHERE id = $1`, p.RaceID); err != nil {
		return nil, err
	}
	return &DeleteRaceEventResponse{Deleted: true}, nil
}

//encore:api public method=DELETE path=/api/race-events
func DeleteRaceEvent(ctx context.Context, p *DeleteRaceEventRequest) (*DeleteRaceEventResponse, error) {
	return DeleteRaceEventCore(ctx, p)
}

// --- Race members ---

// RaceMemberRequest carries event/race/user ids for member mutations.
type RaceMemberRequest struct {
	EventID       string `json:"eventId"`
	RaceID        string `json:"raceId"`
	UserID        string `json:"userId"`
	Authorization string `header:"Authorization"`
}

// AddRaceEventMemberCore registers a participant for a race. The user must be
// an event member first; class restriction and capacity limits are enforced.
func AddRaceEventMemberCore(ctx context.Context, p *RaceMemberRequest) (*RaceEventDetail, error) {
	var userTier sql.NullString
	if err := db.QueryRow(ctx, `SELECT "classTier" FROM "user" WHERE id = $1`, p.UserID).Scan(&userTier); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &errs.Error{Code: errs.NotFound, Message: "user not found"}
		}
		return nil, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var e eventRow
	err = tx.QueryRow(ctx, `SELECT `+eventColumns+` FROM "event" WHERE id = $1`, p.EventID,
	).Scan(&e.ID, &e.Name, &e.Description, &e.OwnerType, &e.OrganizationID,
		&e.OwnerUserID, &e.Status, &e.Tag, &e.DeletedAt, &e.ScoringType, &e.ScoringRulesMode,
		&e.CustomScoringTables, &e.ClassRestriction, &e.GranularParticipation,
		&e.SignupsLocked, &e.ScheduledAt, &e.ParticipantLimit,
		&e.MaxConcurrentRaceParticipations, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "event not found"}
	}
	if err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	var r raceEventRow
	err = tx.QueryRow(ctx, `SELECT `+raceEventColumns+` FROM "race_event" WHERE id = $1`, p.RaceID,
	).Scan(&r.ID, &r.EventID, &r.Name, &r.Sequence, &r.DistanceMeters,
		&r.TrackType, &r.Location, &r.ScoringType, &r.Grade, &r.ClassRestriction,
		&r.StartsAt, &r.EndsAt, &r.ParticipantLimit, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) || r.EventID != p.EventID {
		return nil, &errs.Error{Code: errs.NotFound, Message: "race not found"}
	}
	if err != nil {
		return nil, err
	}
	var memberExists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM "event_member" WHERE "eventId"=$1 AND "userId"=$2)`,
		p.EventID, p.UserID).Scan(&memberExists); err != nil {
		return nil, err
	}
	if !memberExists {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "user is not a member of this event"}
	}
	if !isEligibleTier(userTier, raceRestriction(&r, &e)) {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "participant class tier does not satisfy the race class restriction"}
	}
	var raceMemberExists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM "race_event_member" WHERE "raceEventId"=$1 AND "userId"=$2)`,
		p.RaceID, p.UserID).Scan(&raceMemberExists); err != nil {
		return nil, err
	}
	if !raceMemberExists {
		if r.ParticipantLimit.Valid {
			var currentCount int
			if err := tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM "race_event_member" WHERE "raceEventId"=$1`, p.RaceID,
			).Scan(&currentCount); err != nil {
				return nil, err
			}
			if currentCount >= int(r.ParticipantLimit.Int64) {
				return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "Race participant capacity has been reached",
					Details: detailsMap{"code": CodeRaceParticipantLimitReached, "limit": int(r.ParticipantLimit.Int64), "currentCount": currentCount}}
			}
		}
		if e.MaxConcurrentRaceParticipations.Valid {
			var joinedCount int
			if err := tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM "race_event_member" m JOIN "race_event" r ON r.id = m."raceEventId"
				 WHERE m."userId"=$1 AND r."eventId"=$2`, p.UserID, p.EventID,
			).Scan(&joinedCount); err != nil {
				return nil, err
			}
			if joinedCount >= int(e.MaxConcurrentRaceParticipations.Int64) {
				return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "User maximum race enrollment count reached",
					Details: detailsMap{"code": CodeGranularUserRaceLimitReached, "limit": int(e.MaxConcurrentRaceParticipations.Int64), "currentCount": joinedCount}}
			}
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO "race_event_member" (id, "raceEventId", "userId") VALUES ($1,$2,$3)`,
			"racemember-"+newID()[:8], p.RaceID, p.UserID); err != nil {
			return nil, err
		}
	}
	if err := EnsureEventStandingsRow(ctx, p.EventID, p.UserID, e.ScoringType); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rr, err := requireRaceEvent(ctx, p.EventID, p.RaceID)
	if err != nil {
		return nil, err
	}
	return loadRaceDetail(ctx, rr)
}

//encore:api public method=POST path=/api/race-event-members
func AddRaceEventMember(ctx context.Context, p *RaceMemberRequest) (*RaceEventDetail, error) {
	return AddRaceEventMemberCore(ctx, p)
}

// RemoveRaceEventMemberCore removes a participant from a race. On granular
// events, losing the last race membership also removes event membership.
func RemoveRaceEventMemberCore(ctx context.Context, p *RaceMemberRequest) (*RaceEventDetail, error) {
	e, err := requireEventRow(ctx, p.EventID)
	if err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	if _, err := requireRaceEvent(ctx, p.EventID, p.RaceID); err != nil {
		return nil, err
	}
	var memberExists bool
	if err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM "event_member" WHERE "eventId"=$1 AND "userId"=$2)`,
		p.EventID, p.UserID).Scan(&memberExists); err != nil {
		return nil, err
	}
	if !memberExists {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "user is not a member of this event"}
	}
	if _, err := db.Exec(ctx,
		`DELETE FROM "race_event_member" WHERE "raceEventId"=$1 AND "userId"=$2`, p.RaceID, p.UserID); err != nil {
		return nil, err
	}
	if e.GranularParticipation {
		var activeCount int
		if err := db.QueryRow(ctx,
			`SELECT COUNT(*) FROM "race_event_member" m JOIN "race_event" r ON r.id = m."raceEventId"
			 WHERE m."userId"=$1 AND r."eventId"=$2`, p.UserID, p.EventID,
		).Scan(&activeCount); err != nil {
			return nil, err
		}
		if activeCount == 0 {
			if err := RemoveMemberFromEvent(ctx, p.EventID, p.UserID); err != nil {
				return nil, err
			}
		}
	}
	rr, err := requireRaceEvent(ctx, p.EventID, p.RaceID)
	if err != nil {
		return nil, err
	}
	return loadRaceDetail(ctx, rr)
}

//encore:api public method=DELETE path=/api/race-event-members
func RemoveRaceEventMember(ctx context.Context, p *RaceMemberRequest) (*RaceEventDetail, error) {
	return RemoveRaceEventMemberCore(ctx, p)
}

// RaceMembersQuery lists participants of a race.
type RaceMembersQuery struct {
	EventID string `query:"eventId"`
	RaceID  string `query:"raceId"`
}

// RaceMembersResponse wraps the member list.
type RaceMembersResponse struct {
	Members []RaceEventMemberView `json:"members"`
}

//encore:api public method=GET path=/api/race-event-members-list
func ListRaceEventMembers(ctx context.Context, q *RaceMembersQuery) (*RaceMembersResponse, error) {
	return ListRaceEventMembersCore(ctx, q)
}

// ListRaceEventMembersCore returns registered participants for a race.
func ListRaceEventMembersCore(ctx context.Context, q *RaceMembersQuery) (*RaceMembersResponse, error) {
	if _, err := requireRaceEvent(ctx, q.EventID, q.RaceID); err != nil {
		return nil, err
	}
	members, err := loadRaceMembers(ctx, q.RaceID)
	if err != nil {
		return nil, err
	}
	return &RaceMembersResponse{Members: members}, nil
}

// --- Join / leave (self-service) ---

// RaceJoinRequest carries event/race ids plus the caller's auth header.
type RaceJoinRequest struct {
	EventID       string `json:"eventId"`
	RaceID        string `json:"raceId"`
	Authorization string `header:"Authorization"`
}

// JoinRaceEventCore lets the caller join a race. On granular events the
// caller is auto-joined to the parent event.
func JoinRaceEventCore(ctx context.Context, p *RaceJoinRequest) (*RaceEventDetail, error) {
	actor, err := auth.ResolveActor(ctx, p.Authorization)
	if err != nil {
		return nil, err
	}
	userID := actor.UserID
	var userTier sql.NullString
	if err := db.QueryRow(ctx, `SELECT "classTier" FROM "user" WHERE id = $1`, userID).Scan(&userTier); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &errs.Error{Code: errs.NotFound, Message: "user not found"}
		}
		return nil, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var e eventRow
	err = tx.QueryRow(ctx, `SELECT `+eventColumns+` FROM "event" WHERE id = $1`, p.EventID,
	).Scan(&e.ID, &e.Name, &e.Description, &e.OwnerType, &e.OrganizationID,
		&e.OwnerUserID, &e.Status, &e.Tag, &e.DeletedAt, &e.ScoringType, &e.ScoringRulesMode,
		&e.CustomScoringTables, &e.ClassRestriction, &e.GranularParticipation,
		&e.SignupsLocked, &e.ScheduledAt, &e.ParticipantLimit,
		&e.MaxConcurrentRaceParticipations, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "event not found"}
	}
	if err != nil {
		return nil, err
	}
	if e.SignupsLocked {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "signups are locked for this event"}
	}
	var r raceEventRow
	err = tx.QueryRow(ctx, `SELECT `+raceEventColumns+` FROM "race_event" WHERE id = $1`, p.RaceID,
	).Scan(&r.ID, &r.EventID, &r.Name, &r.Sequence, &r.DistanceMeters,
		&r.TrackType, &r.Location, &r.ScoringType, &r.Grade, &r.ClassRestriction,
		&r.StartsAt, &r.EndsAt, &r.ParticipantLimit, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) || r.EventID != p.EventID {
		return nil, &errs.Error{Code: errs.NotFound, Message: "race not found"}
	}
	if err != nil {
		return nil, err
	}
	var memberExists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM "event_member" WHERE "eventId"=$1 AND "userId"=$2)`,
		p.EventID, userID).Scan(&memberExists); err != nil {
		return nil, err
	}
	if !memberExists && !e.GranularParticipation {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "user is not a member of this event"}
	}
	if !isEligibleTier(userTier, raceRestriction(&r, &e)) {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "participant class tier does not satisfy the race class restriction"}
	}
	var raceMemberExists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM "race_event_member" WHERE "raceEventId"=$1 AND "userId"=$2)`,
		p.RaceID, userID).Scan(&raceMemberExists); err != nil {
		return nil, err
	}
	if !raceMemberExists {
		if r.ParticipantLimit.Valid {
			var currentCount int
			if err := tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM "race_event_member" WHERE "raceEventId"=$1`, p.RaceID,
			).Scan(&currentCount); err != nil {
				return nil, err
			}
			if currentCount >= int(r.ParticipantLimit.Int64) {
				return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "Race participant capacity has been reached",
					Details: detailsMap{"code": CodeRaceParticipantLimitReached, "limit": int(r.ParticipantLimit.Int64), "currentCount": currentCount}}
			}
		}
		if e.MaxConcurrentRaceParticipations.Valid {
			var joinedCount int
			if err := tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM "race_event_member" m JOIN "race_event" r ON r.id = m."raceEventId"
				 WHERE m."userId"=$1 AND r."eventId"=$2`, userID, p.EventID,
			).Scan(&joinedCount); err != nil {
				return nil, err
			}
			if joinedCount >= int(e.MaxConcurrentRaceParticipations.Int64) {
				return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "User maximum race enrollment count reached",
					Details: detailsMap{"code": CodeGranularUserRaceLimitReached, "limit": int(e.MaxConcurrentRaceParticipations.Int64), "currentCount": joinedCount}}
			}
		}
		if !memberExists && e.GranularParticipation {
			if _, err := tx.Exec(ctx,
				`INSERT INTO "event_member" (id, "eventId", "userId") VALUES ($1,$2,$3)`,
				"eventmember-"+newID()[:8], p.EventID, userID); err != nil {
				return nil, err
			}
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO "race_event_member" (id, "raceEventId", "userId") VALUES ($1,$2,$3)`,
			"racemember-"+newID()[:8], p.RaceID, userID); err != nil {
			return nil, err
		}
	}
	if err := EnsureEventStandingsRow(ctx, p.EventID, userID, e.ScoringType); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	rr, err := requireRaceEvent(ctx, p.EventID, p.RaceID)
	if err != nil {
		return nil, err
	}
	return loadRaceDetail(ctx, rr)
}

//encore:api public method=POST path=/api/race-event-join
func JoinRaceEvent(ctx context.Context, p *RaceJoinRequest) (*RaceEventDetail, error) {
	return JoinRaceEventCore(ctx, p)
}

// LeaveRaceEventCore lets the caller leave a race. Self-service leave is
// blocked while signups are locked.
func LeaveRaceEventCore(ctx context.Context, p *RaceJoinRequest) (*RaceEventDetail, error) {
	e, err := requireEventRow(ctx, p.EventID)
	if err != nil {
		return nil, err
	}
	if e.SignupsLocked {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "signups are locked for this event"}
	}
	if _, err := requireRaceEvent(ctx, p.EventID, p.RaceID); err != nil {
		return nil, err
	}
	actor, err := auth.ResolveActor(ctx, p.Authorization)
	if err != nil {
		return nil, err
	}
	userID := actor.UserID
	var memberExists bool
	if err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM "event_member" WHERE "eventId"=$1 AND "userId"=$2)`,
		p.EventID, userID).Scan(&memberExists); err != nil {
		return nil, err
	}
	if !memberExists {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "user is not a member of this event"}
	}
	if _, err := db.Exec(ctx,
		`DELETE FROM "race_event_member" WHERE "raceEventId"=$1 AND "userId"=$2`, p.RaceID, userID); err != nil {
		return nil, err
	}
	if e.GranularParticipation {
		var activeCount int
		if err := db.QueryRow(ctx,
			`SELECT COUNT(*) FROM "race_event_member" m JOIN "race_event" r ON r.id = m."raceEventId"
			 WHERE m."userId"=$1 AND r."eventId"=$2`, userID, p.EventID,
		).Scan(&activeCount); err != nil {
			return nil, err
		}
		if activeCount == 0 {
			if err := RemoveMemberFromEvent(ctx, p.EventID, userID); err != nil {
				return nil, err
			}
		}
	}
	rr, err := requireRaceEvent(ctx, p.EventID, p.RaceID)
	if err != nil {
		return nil, err
	}
	return loadRaceDetail(ctx, rr)
}

//encore:api public method=DELETE path=/api/race-event-join
func LeaveRaceEvent(ctx context.Context, p *RaceJoinRequest) (*RaceEventDetail, error) {
	return LeaveRaceEventCore(ctx, p)
}

// raceRestriction returns the effective class restriction for a race,
// falling back to the parent event's restriction.
//
// Mirrors the `race.classRestriction ?? event.classRestriction` fallback in
// ts-legacy/eventmanager/raceevents.ts.
func raceRestriction(r *raceEventRow, e *eventRow) *ClassTier {
	if r.ClassRestriction.Valid {
		t := ClassTier(r.ClassRestriction.String)
		return &t
	}
	if e.ClassRestriction.Valid {
		t := ClassTier(e.ClassRestriction.String)
		return &t
	}
	return nil
}

// isEligibleTier adapts a nullable DB tier to IsEligible.
func isEligibleTier(userTier sql.NullString, restriction *ClassTier) bool {
	var tier *ClassTier
	if userTier.Valid {
		t := ClassTier(userTier.String)
		tier = &t
	}
	return IsEligible(tier, restriction)
}
