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
	"encore.app/shared"

	"github.com/sqlc-dev/pqtype"
)

// Scoring types, mirroring ts-legacy/lib/constants.ts.
const (
	ScoringPoints = 1
	ScoringLadder = 2
)

var scoringLabels = map[int]string{
	ScoringPoints: "points-based",
	ScoringLadder: "ladder-elo",
}

// scoringLabel mirrors the TS SCORING_LABELS lookup with "unknown" fallback.
func scoringLabel(scoringType int) string {
	if label, ok := scoringLabels[scoringType]; ok {
		return label
	}
	return "unknown"
}

// truncate clamps s to max bytes (TS slice(0, n) clamps; Go panics).
func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

// newID generates a random UUIDv4 string for row ids.
func newID() string {
	return shared.NewID()
}

// isoTime formats a timestamp as an ISO-8601 UTC string (mirrors TS toISOString).
func isoTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// --- API-facing views (mirror ts-legacy/eventmanager/events.ts) ---

// EventListItem is the summary row returned by list endpoints.
type EventListItem struct {
	ID                              string  `json:"id"`
	Name                            string  `json:"name"`
	Description                     *string `json:"description"`
	OwnerType                       string  `json:"ownerType"`
	OrganizationID                  *string `json:"organizationId"`
	OwnerUserID                     *string `json:"ownerUserId"`
	Status                          string  `json:"status"`
	Tag                             string  `json:"tag"`
	DeletedAt                       *string `json:"deletedAt,omitempty"`
	ScoringType                     int     `json:"scoringType"`
	ScoringTypeLabel                string  `json:"scoringTypeLabel"`
	ClassRestriction                *string `json:"classRestriction"`
	GranularParticipation           bool    `json:"granularParticipation"`
	SignupsLocked                   bool    `json:"signupsLocked"`
	ScheduledAt                     *string `json:"scheduledAt"`
	ParticipantLimit                *int    `json:"participantLimit"`
	MaxConcurrentRaceParticipations *int    `json:"maxConcurrentRaceParticipations"`
	RaceCount                       int     `json:"raceCount"`
	MemberCount                     int     `json:"memberCount"`
	CreatedAt                       string  `json:"createdAt"`
	UpdatedAt                       string  `json:"updatedAt"`
}

// EventMemberView is a single event participant.
type EventMemberView struct {
	UserID    string  `json:"userId"`
	Name      string  `json:"name"`
	ClassTier *string `json:"classTier"`
}

// EventScheduleView is a single schedule slot.
type EventScheduleView struct {
	ID       string  `json:"id"`
	Title    *string `json:"title"`
	StartsAt string  `json:"startsAt"`
	EndsAt   *string `json:"endsAt"`
	Location *string `json:"location"`
}

// PointsEntryView is one row of the points overview.
type PointsEntryView struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Points int    `json:"points"`
}

// LadderEntryView is one row of the ladder overview.
type LadderEntryView struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Elo    int    `json:"elo"`
	Wins   int    `json:"wins"`
	Losses int    `json:"losses"`
	Rank   int    `json:"rank"`
}

// RaceEventMemberView is a single race participant.
type RaceEventMemberView struct {
	UserID    string  `json:"userId"`
	Name      string  `json:"name"`
	ClassTier *string `json:"classTier"`
}

// RaceEventView is a race belonging to an event.
type RaceEventView struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Sequence         int                   `json:"sequence"`
	DistanceMeters   int                   `json:"distanceMeters"`
	TrackType        string                `json:"trackType"`
	Location         string                `json:"location"`
	ScoringType      *int                  `json:"scoringType"`
	Grade            *string               `json:"grade"`
	ClassRestriction *string               `json:"classRestriction"`
	StartsAt         *string               `json:"startsAt"`
	EndsAt           *string               `json:"endsAt"`
	ParticipantLimit *int                  `json:"participantLimit"`
	Members          []RaceEventMemberView `json:"members"`
}

// EventDetail is the full event payload.
//
// Mirrors ts-legacy/eventmanager/events.ts EventDetail.
type EventDetail struct {
	ID                              string             `json:"id"`
	Name                            string             `json:"name"`
	Description                     *string            `json:"description"`
	OwnerType                       string             `json:"ownerType"`
	OrganizationID                  *string            `json:"organizationId"`
	OwnerUserID                     *string            `json:"ownerUserId"`
	Status                          string             `json:"status"`
	Tag                             string             `json:"tag"`
	DeletedAt                       *string            `json:"deletedAt,omitempty"`
	ScoringType                     int                `json:"scoringType"`
	ScoringTypeLabel                string             `json:"scoringTypeLabel"`
	ScoringRulesMode                *string            `json:"scoringRulesMode"`
	CustomScoringTables             json.RawMessage    `json:"customScoringTables"`
	ClassRestriction                *string            `json:"classRestriction"`
	GranularParticipation           bool               `json:"granularParticipation"`
	SignupsLocked                   bool               `json:"signupsLocked"`
	ScheduledAt                     *string            `json:"scheduledAt"`
	ParticipantLimit                *int               `json:"participantLimit"`
	MaxConcurrentRaceParticipations *int               `json:"maxConcurrentRaceParticipations"`
	RaceEvents                      []RaceEventView    `json:"raceEvents"`
	Members                         []EventMemberView  `json:"members"`
	Schedules                       []EventScheduleView `json:"schedules"`
	PointsOverview                  []PointsEntryView  `json:"pointsOverview"`
	LadderOverview                  []LadderEntryView  `json:"ladderOverview"`
	CreatedAt                       string             `json:"createdAt"`
	UpdatedAt                       string             `json:"updatedAt"`
}

// nullString converts sql.NullString to *string.
func nullString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

// nullInt converts sql.NullInt64 to *int.
func nullInt(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	n := int(ni.Int64)
	return &n
}

// nullTime converts *time.Time to an ISO *string.
func nullTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := isoTime(*t)
	return &s
}

// classTierPtr converts a nullable class-tier column to the *string form used
// in API views (empty treated as unset).
func classTierPtr(ns sql.NullString) *string {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	s := ns.String
	return &s
}

// toClassTier converts a *string view value to the classtier type for eligibility checks.
func toClassTier(s *string) *ClassTier {
	if s == nil || *s == "" {
		return nil
	}
	t := ClassTier(*s)
	return &t
}

// eventRow holds the raw event columns for loadEvent.
type eventRow struct {
	ID                              string
	Name                            string
	Description                     sql.NullString
	OwnerType                       string
	OrganizationID                  sql.NullString
	OwnerUserID                     sql.NullString
	Status                          string
	Tag                             string
	DeletedAt                       *time.Time
	ScoringType                     int
	ScoringRulesMode                sql.NullString
	CustomScoringTables             []byte
	ClassRestriction                sql.NullString
	GranularParticipation           bool
	SignupsLocked                   bool
	ScheduledAt                     *time.Time
	ParticipantLimit                sql.NullInt64
	MaxConcurrentRaceParticipations sql.NullInt64
	CreatedAt                       time.Time
	UpdatedAt                       time.Time
}

// toEventRow maps a sqlc event row onto the local eventRow.
func toEventRow(r sqlc.GetEventRowRow) *eventRow {
	var customTables []byte
	if r.CustomScoringTables.Valid {
		customTables = r.CustomScoringTables.RawMessage
	}
	return &eventRow{
		ID: r.ID, Name: r.Name, Description: r.Description,
		OwnerType: stringFromAny(r.OwnerType),
		OrganizationID: r.OrganizationId, OwnerUserID: r.OwnerUserId,
		Status: r.Status, Tag: r.Tag, DeletedAt: timePtrFromNull(r.DeletedAt),
		ScoringType: int(r.ScoringType), ScoringRulesMode: r.ScoringRulesMode,
		CustomScoringTables: customTables,
		ClassRestriction: nullStringFromAny(r.ClassRestriction),
		GranularParticipation: r.GranularParticipation, SignupsLocked: r.SignupsLocked,
		ScheduledAt: timePtrFromNull(r.ScheduledAt),
		ParticipantLimit: sql.NullInt64{Int64: int64(r.ParticipantLimit.Int32), Valid: r.ParticipantLimit.Valid},
		MaxConcurrentRaceParticipations: sql.NullInt64{Int64: int64(r.MaxConcurrentRaceParticipations.Int32), Valid: r.MaxConcurrentRaceParticipations.Valid},
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// LoadEvent returns a single event with members, schedules and the scoring
// overview matching its scoring type.
//
// Mirrors ts-legacy/eventmanager/events.ts loadEvent.
func LoadEvent(ctx context.Context, id string) (*EventDetail, error) {
	e, err := requireEventRow(ctx, id)
	if err != nil {
		return nil, err
	}

	// Races.
	type raceRow struct {
		ID               string
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
	}
	var races []raceRow
	rrows, err := q().ListEventRaces(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, rr := range rrows {
		races = append(races, raceRow{
			ID: rr.ID, Name: rr.Name, Sequence: int(rr.Sequence),
			DistanceMeters: int(rr.DistanceMeters), TrackType: rr.TrackType, Location: rr.Location,
			ScoringType: sql.NullInt64{Int64: int64(rr.ScoringType.Int16), Valid: rr.ScoringType.Valid},
			Grade: rr.Grade, ClassRestriction: nullStringFromAny(rr.ClassRestriction),
			StartsAt: timePtrFromNull(rr.StartsAt), EndsAt: timePtrFromNull(rr.EndsAt),
			ParticipantLimit: sql.NullInt64{Int64: int64(rr.ParticipantLimit.Int32), Valid: rr.ParticipantLimit.Valid},
		})
	}

	// Race members with display names, grouped by race.
	raceMembers := map[string][]RaceEventMemberView{}
	activeUserIDs := map[string]bool{}
	if len(races) > 0 {
		mrows, err := q().ListRaceMembersByEvent(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, m := range mrows {
			tier := nullStringFromAny(m.ClassTier)
			raceMembers[m.RaceEventId] = append(raceMembers[m.RaceEventId], RaceEventMemberView{
				UserID: m.UserId, Name: m.VrchatUsername, ClassTier: classTierPtr(tier),
			})
			activeUserIDs[m.UserId] = true
		}
	}

	// Event members; granular events only surface members active in a race.
	type memberRow struct {
		UserID string
		Name   string
		Tier   sql.NullString
	}
	var members []memberRow
	emrows, err := q().ListEventMembersByEvent(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, em := range emrows {
		m := memberRow{UserID: em.UserId, Name: em.VrchatUsername, Tier: nullStringFromAny(em.ClassTier)}
		if e.GranularParticipation && !activeUserIDs[m.UserID] {
			continue
		}
		members = append(members, m)
	}

	// Schedules.
	var schedules []EventScheduleView
	srows, err := q().ListEventSchedules(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, sr := range srows {
		schedules = append(schedules, EventScheduleView{
			ID: sr.ID, Title: nullString(sr.Title), StartsAt: isoTime(sr.StartsAt),
			EndsAt: nullTime(timePtrFromNull(sr.EndsAt)), Location: nullString(sr.Location),
		})
	}

	// Scoring overviews matching the event's scoring type (the other stays null).
	var pointsOverview []PointsEntryView
	var ladderOverview []LadderEntryView
	if e.ScoringType == ScoringPoints {
		pointsOverview = []PointsEntryView{}
		prows, err := q().ListPointsOverview(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, pr := range prows {
			pointsOverview = append(pointsOverview, PointsEntryView{
				UserID: pr.UserId, Name: pr.VrchatUsername, Points: int(pr.Points),
			})
		}
	} else if e.ScoringType == ScoringLadder {
		ladderOverview = []LadderEntryView{}
		lrows, err := q().ListLadderOverview(ctx, id)
		if err != nil {
			return nil, err
		}
		rank := 0
		for _, lr := range lrows {
			rank++
			ladderOverview = append(ladderOverview, LadderEntryView{
				UserID: lr.UserId, Name: lr.VrchatUsername,
				Elo: int(lr.Elo), Wins: int(lr.Wins), Losses: int(lr.Losses), Rank: rank,
			})
		}
	}

	var customTables json.RawMessage
	if len(e.CustomScoringTables) > 0 {
		if json.Valid(e.CustomScoringTables) {
			customTables = e.CustomScoringTables
		}
	}

	raceViews := make([]RaceEventView, 0, len(races))
	for _, r := range races {
		mems := raceMembers[r.ID]
		if mems == nil {
			mems = []RaceEventMemberView{}
		}
		raceViews = append(raceViews, RaceEventView{
			ID: r.ID, Name: r.Name, Sequence: r.Sequence,
			DistanceMeters: r.DistanceMeters, TrackType: r.TrackType, Location: r.Location,
			ScoringType: nullInt(r.ScoringType), Grade: nullString(r.Grade),
			ClassRestriction: classTierPtr(r.ClassRestriction),
			StartsAt:         nullTime(r.StartsAt), EndsAt: nullTime(r.EndsAt),
			ParticipantLimit: nullInt(r.ParticipantLimit), Members: mems,
		})
	}
	memberViews := make([]EventMemberView, 0, len(members))
	for _, m := range members {
		memberViews = append(memberViews, EventMemberView{
			UserID: m.UserID, Name: m.Name, ClassTier: classTierPtr(m.Tier),
		})
	}
	if schedules == nil {
		schedules = []EventScheduleView{}
	}

	return &EventDetail{
		ID: e.ID, Name: e.Name, Description: nullString(e.Description),
		OwnerType: e.OwnerType, OrganizationID: nullString(e.OrganizationID),
		OwnerUserID: nullString(e.OwnerUserID), Status: e.Status, Tag: e.Tag,
		DeletedAt: nullTime(e.DeletedAt),
		ScoringType: e.ScoringType, ScoringTypeLabel: scoringLabel(e.ScoringType),
		ScoringRulesMode: nullString(e.ScoringRulesMode), CustomScoringTables: customTables,
		ClassRestriction: classTierPtr(e.ClassRestriction),
		GranularParticipation: e.GranularParticipation, SignupsLocked: e.SignupsLocked,
		ScheduledAt: nullTime(e.ScheduledAt),
		ParticipantLimit: nullInt(e.ParticipantLimit),
		MaxConcurrentRaceParticipations: nullInt(e.MaxConcurrentRaceParticipations),
		RaceEvents: raceViews, Members: memberViews, Schedules: schedules,
		PointsOverview: pointsOverview, LadderOverview: ladderOverview,
		CreatedAt: isoTime(e.CreatedAt), UpdatedAt: isoTime(e.UpdatedAt),
	}, nil
}

// EnsureEventStandingsRow seeds the scoring record for a member, idempotently.
//
// Mirrors ts-legacy/eventmanager/events.ts ensureEventStandingsRow.
func EnsureEventStandingsRow(ctx context.Context, eventID, userID string, scoringType int) error {
	now := time.Now().UTC()
	if scoringType == ScoringPoints {
		return q().InsertPointsStandingsRow(ctx, sqlc.InsertPointsStandingsRowParams{
			ID: newID(), EventId: eventID, UserId: userID, CreatedAt: now,
		})
	}
	return q().InsertLadderStandingsRow(ctx, sqlc.InsertLadderStandingsRowParams{
		ID: newID(), EventId: eventID, UserId: userID, Elo: int32(LadderStartingElo), CreatedAt: now,
	})
}

// --- Create ---

// CreateEventRequest carries the create payload plus the auth header.
//
// Mirrors ts-legacy/eventmanager/events.ts CreateEventParams (POST /api/events).
type CreateEventRequest struct {
	Authorization                     string          `header:"Authorization"`
	Name                              string          `json:"name"`
	Description                       *string         `json:"description,omitempty"`
	OwnerType                         string          `json:"ownerType"`
	OrganizationID                    *string         `json:"organizationId,omitempty"`
	OwnerUserID                       *string         `json:"ownerUserId,omitempty"`
	Tag                               *string         `json:"tag,omitempty"`
	ScoringType                       int             `json:"scoringType"`
	ScoringRulesMode                  *string         `json:"scoringRulesMode,omitempty"`
	CustomScoringTables               json.RawMessage `json:"customScoringTables,omitempty"`
	ClassRestriction                  *string         `json:"classRestriction,omitempty"`
	GranularParticipation             bool            `json:"granularParticipation,omitempty"`
	ScheduledAt                       *string         `json:"scheduledAt,omitempty"`
	ParticipantLimit                  OptInt          `json:"participantLimit,omitempty"`
	MaxConcurrentRaceParticipations   OptInt          `json:"maxConcurrentRaceParticipations,omitempty"`
}

// CreateEventCore creates an event and returns its detail.
func CreateEventCore(ctx context.Context, p *CreateEventRequest) (*EventDetail, error) {
	if p.Name == "" {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "name is required"}
	}
	if p.OwnerType != "ORGANIZATION" && p.OwnerType != "USER" {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "ownerType must be ORGANIZATION or USER"}
	}

	var organizationID, ownerUserID *string
	if p.OwnerType == "ORGANIZATION" {
		if p.OrganizationID == nil || *p.OrganizationID == "" {
			return nil, &errs.Error{Code: errs.InvalidArgument, Message: "organizationId is required for organization-owned events"}
		}
		if _, _, err := auth.RequirePermission(ctx, p.Authorization, *p.OrganizationID, auth.ResourceEvent, auth.ActionCreate); err != nil {
			return nil, err
		}
		orgExists, err := q().OrgExists(ctx, *p.OrganizationID)
		if err != nil {
			return nil, err
		}
		if !orgExists {
			return nil, &errs.Error{Code: errs.NotFound, Message: "organization not found"}
		}
		organizationID = p.OrganizationID
	} else {
		actor, err := auth.ResolveActor(ctx, p.Authorization)
		if err != nil {
			return nil, err
		}
		owner := actor.UserID
		if p.OwnerUserID != nil && *p.OwnerUserID != "" {
			owner = *p.OwnerUserID
		}
		if owner != actor.UserID && !auth.IsSiteAdmin(actor.SiteRole) {
			return nil, &errs.Error{Code: errs.PermissionDenied, Message: "only a site administrator can create an event on behalf of another user"}
		}
		ownerExists, err := q().UserExists(ctx, owner)
		if err != nil {
			return nil, err
		}
		if !ownerExists {
			return nil, &errs.Error{Code: errs.NotFound, Message: "owner user not found"}
		}
		ownerUserID = &owner
	}

	var scoringRulesMode *string
	var customTables any
	if p.ScoringType == ScoringPoints {
		mode := "STANDARD"
		if p.ScoringRulesMode != nil && *p.ScoringRulesMode != "" {
			mode = *p.ScoringRulesMode
		}
		if mode == "CUSTOM" {
			if len(p.CustomScoringTables) == 0 || string(p.CustomScoringTables) == "null" {
				return nil, &errs.Error{Code: errs.InvalidArgument, Message: "customScoringTables is required when scoringRulesMode is CUSTOM"}
			}
			var v any
			if err := json.Unmarshal(p.CustomScoringTables, &v); err != nil {
				return nil, &errs.Error{Code: errs.InvalidArgument, Message: "customScoringTables must be an object"}
			}
			if _, err := ValidateCustomScoringTables(v); err != nil {
				return nil, err
			}
			customTables = v
		} else {
			mode = "STANDARD"
		}
		scoringRulesMode = &mode
	}

	isGranular := p.GranularParticipation
	var participantLimit, maxConcurrent *int
	if p.ParticipantLimit.Set {
		participantLimit = p.ParticipantLimit.Value
	}
	if p.MaxConcurrentRaceParticipations.Set {
		maxConcurrent = p.MaxConcurrentRaceParticipations.Value
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

	var scheduledAt *time.Time
	if p.ScheduledAt != nil && *p.ScheduledAt != "" {
		t, err := time.Parse(time.RFC3339Nano, *p.ScheduledAt)
		if err != nil {
			return nil, &errs.Error{Code: errs.InvalidArgument, Message: "scheduledAt must be an ISO-8601 timestamp"}
		}
		utc := t.UTC()
		scheduledAt = &utc
	}

	var classRestriction *string
	if p.ClassRestriction != nil && *p.ClassRestriction != "" {
		classRestriction = p.ClassRestriction
	}

	var description *string
	if p.Description != nil {
		description = p.Description
	}

	var customTablesJSON any
	if customTables != nil {
		raw, err := json.Marshal(customTables)
		if err != nil {
			return nil, err
		}
		customTablesJSON = string(raw)
	}

	tag := "COMMUNITY"
	if p.Tag != nil && *p.Tag != "" {
		tag = *p.Tag
	}
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

	now := time.Now().UTC()
	id := newID()
	var customTablesParam pqtype.NullRawMessage
	if customTablesJSON != nil {
		if s, ok := customTablesJSON.(string); ok {
			customTablesParam = pqtype.NullRawMessage{RawMessage: []byte(s), Valid: true}
		}
	}
	var classRestr any
	if classRestriction != nil {
		classRestr = *classRestriction
	}
	var participantLimitNull, maxConcurrentNull sql.NullInt32
	if participantLimit != nil {
		participantLimitNull = sql.NullInt32{Int32: int32(*participantLimit), Valid: true}
	}
	if maxConcurrent != nil {
		maxConcurrentNull = sql.NullInt32{Int32: int32(*maxConcurrent), Valid: true}
	}
	if err := q().CreateEvent(ctx, sqlc.CreateEventParams{
		ID: id, Name: truncate(p.Name, 255), Description: nullStringFromPtr(description),
		OwnerType: p.OwnerType, OrganizationId: nullStringFromPtr(organizationID),
		OwnerUserId: nullStringFromPtr(ownerUserID), Tag: tag,
		ScoringType: int16(p.ScoringType), ScoringRulesMode: nullStringFromPtr(scoringRulesMode),
		CustomScoringTables: customTablesParam, ClassRestriction: classRestr,
		GranularParticipation: isGranular, ScheduledAt: nullTimeFromPtr(scheduledAt),
		ParticipantLimit: participantLimitNull, MaxConcurrentRaceParticipations: maxConcurrentNull,
		CreatedAt: now,
	}); err != nil {
		return nil, err
	}
	return LoadEvent(ctx, id)
}

//encore:api public method=POST path=/api/events
func CreateEvent(ctx context.Context, p *CreateEventRequest) (*EventDetail, error) {
	return CreateEventCore(ctx, p)
}

// --- List ---

// PurgeExpiredDeletedEvents permanently deletes events in PENDING_DELETION state
// that were deleted more than 7 days ago.
func PurgeExpiredDeletedEvents(ctx context.Context) error {
	threshold := time.Now().UTC().AddDate(0, 0, -7)
	return q().PurgeExpiredDeletedEvents(ctx, sql.NullTime{Time: threshold, Valid: true})
}

// ListEventsQuery carries optional filters plus pagination.
//
// Mirrors ts-legacy/eventmanager/events.ts ListEventsParams (GET /api/events).
type ListEventsQuery struct {
	OrganizationID   string `query:"organizationId"`
	ClassRestriction string `query:"classRestriction"`
	Status           string `query:"status"`
	Tag              string `query:"tag"`
	IncludeDeleted   bool   `query:"includeDeleted"`
	Limit            int    `query:"limit"`
	Offset           int    `query:"offset"`
}

// ListEventsResponse carries the page plus the total count.
type ListEventsResponse struct {
	Events []EventListItem `json:"events"`
	Total  int             `json:"total"`
}

// toListItem maps a sqlc list row onto the public EventListItem.
func toListItem(r sqlc.ListEventsRow) EventListItem {
	return EventListItem{
		ID: r.ID, Name: r.Name, Description: nullString(r.Description),
		OwnerType: stringFromAny(r.OwnerType),
		OrganizationID: nullString(r.OrganizationId),
		OwnerUserID: nullString(r.OwnerUserId),
		Status: r.Status, Tag: r.Tag, DeletedAt: nullTime(timePtrFromNull(r.DeletedAt)),
		ScoringType: int(r.ScoringType), ScoringTypeLabel: scoringLabel(int(r.ScoringType)),
		ClassRestriction: classTierPtr(nullStringFromAny(r.ClassRestriction)),
		GranularParticipation: r.GranularParticipation, SignupsLocked: r.SignupsLocked,
		ScheduledAt: nullTime(timePtrFromNull(r.ScheduledAt)),
		ParticipantLimit: nullInt(sql.NullInt64{Int64: int64(r.ParticipantLimit.Int32), Valid: r.ParticipantLimit.Valid}),
		MaxConcurrentRaceParticipations: nullInt(sql.NullInt64{Int64: int64(r.MaxConcurrentRaceParticipations.Int32), Valid: r.MaxConcurrentRaceParticipations.Valid}),
		RaceCount: int(r.Count), MemberCount: int(r.Count_2),
		CreatedAt: isoTime(r.CreatedAt), UpdatedAt: isoTime(r.UpdatedAt),
	}
}

// toPublicListItem adapts a public-listing row (identical columns) to the
// shared list-row shape.
func toPublicListItem(r sqlc.ListPublicEventsRow) sqlc.ListEventsRow {
	return sqlc.ListEventsRow{
		ID: r.ID, Name: r.Name, Description: r.Description, OwnerType: r.OwnerType,
		OrganizationId: r.OrganizationId, OwnerUserId: r.OwnerUserId,
		Status: r.Status, Tag: r.Tag, DeletedAt: r.DeletedAt,
		ScoringType: r.ScoringType, ClassRestriction: r.ClassRestriction,
		GranularParticipation: r.GranularParticipation, SignupsLocked: r.SignupsLocked,
		ScheduledAt: r.ScheduledAt, ParticipantLimit: r.ParticipantLimit,
		MaxConcurrentRaceParticipations: r.MaxConcurrentRaceParticipations,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		Count: r.Count, Count_2: r.Count_2,
	}
}

// ListEventsCore lists events with optional filters.
func ListEventsCore(ctx context.Context, q *ListEventsQuery) (*ListEventsResponse, error) {
	_ = PurgeExpiredDeletedEvents(ctx)

	qq := sqlc.New(std())
	total, err := qq.CountEvents(ctx, sqlc.CountEventsParams{
		Column1: q.OrganizationID,
		Column2: q.ClassRestriction,
		Column3: q.Tag,
		Column4: q.Status,
		Column5: q.IncludeDeleted,
	})
	if err != nil {
		return nil, err
	}
	rows, err := qq.ListEvents(ctx, sqlc.ListEventsParams{
		Column1: q.OrganizationID,
		Column2: q.ClassRestriction,
		Column3: q.Tag,
		Column4: q.Status,
		Column5: q.IncludeDeleted,
		Column6: int32(q.Limit),
		Column7: int32(q.Offset),
	})
	if err != nil {
		return nil, err
	}
	items := []EventListItem{}
	for _, r := range rows {
		items = append(items, toListItem(r))
	}
	return &ListEventsResponse{Events: items, Total: int(total)}, nil
}

//encore:api public method=GET path=/api/events
func ListEvents(ctx context.Context, q *ListEventsQuery) (*ListEventsResponse, error) {
	return ListEventsCore(ctx, q)
}

// --- Public list ---

// ListPublicEventsQuery carries pagination for the public listing.
type ListPublicEventsQuery struct {
	Limit  int `query:"limit"`
	Offset int `query:"offset"`
}

// ListPublicEventsCore lists non-draft, active events (PENDING, ONGOING, CONCLUDED).
func ListPublicEventsCore(ctx context.Context, q *ListPublicEventsQuery) (*ListEventsResponse, error) {
	_ = PurgeExpiredDeletedEvents(ctx)
	limit := q.Limit
	if limit == 0 {
		limit = 10
	}
	qq := sqlc.New(std())
	total, err := qq.CountPublicEvents(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := qq.ListPublicEvents(ctx, sqlc.ListPublicEventsParams{
		Column1: int32(limit),
		Column2: int32(q.Offset),
	})
	if err != nil {
		return nil, err
	}
	items := []EventListItem{}
	for _, r := range rows {
		items = append(items, toListItem(toPublicListItem(r)))
	}
	return &ListEventsResponse{Events: items, Total: int(total)}, nil
}

//encore:api public method=GET path=/api/events-public
func ListPublicEvents(ctx context.Context, q *ListPublicEventsQuery) (*ListEventsResponse, error) {
	return ListPublicEventsCore(ctx, q)
}

// --- Get ---

//encore:api public method=GET path=/api/events/:id
func GetEvent(ctx context.Context, id string) (*EventDetail, error) {
	return LoadEvent(ctx, id)
}

// --- Admins ---

// EventAdminView is one event administrator.
type EventAdminView struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
}

// ListEventAdminsResponse carries the admin list.
type ListEventAdminsResponse struct {
	Admins []EventAdminView `json:"admins"`
}

// ListEventAdminsCore lists event administrators.
//
// Mirrors ts-legacy/eventmanager/events.ts listEventAdmins.
func ListEventAdminsCore(ctx context.Context, eventID string) (*ListEventAdminsResponse, error) {
	rows, err := q().ListEventAdmins(ctx, eventID)
	if err != nil {
		return nil, err
	}
	admins := []EventAdminView{}
	for _, r := range rows {
		admins = append(admins, EventAdminView{UserID: r.UserId, Name: r.VrchatUsername})
	}
	return &ListEventAdminsResponse{Admins: admins}, nil
}

//encore:api public method=GET path=/api/event-admins/:eventID
func ListEventAdmins(ctx context.Context, eventID string) (*ListEventAdminsResponse, error) {
	return ListEventAdminsCore(ctx, eventID)
}

// AddEventAdminRequest carries the event/user ids plus the auth header.
// The ids travel in the body (rather than :id path params) because this
// Encore version only accepts scalar params alongside path params.
//
// Mirrors ts-legacy/eventmanager/events.ts addEventAdmin (POST /api/events/:id/admins).
type AddEventAdminRequest struct {
	EventID       string `json:"eventId"`
	UserID        string `json:"userId"`
	Authorization string `header:"Authorization"`
}

// AddEventAdminResponse reports success.
type AddEventAdminResponse struct {
	Success bool `json:"success"`
}

// AddEventAdminCore adds an administrator to an event (idempotent).
func AddEventAdminCore(ctx context.Context, p *AddEventAdminRequest) (*AddEventAdminResponse, error) {
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	userExists, err := q().UserExists(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	if !userExists {
		return nil, &errs.Error{Code: errs.NotFound, Message: "user not found"}
	}
	// Idempotent: ignore duplicate admin rows.
	if err := q().InsertEventAdmin(ctx, sqlc.InsertEventAdminParams{
		ID: newID(), EventId: p.EventID, UserId: p.UserID, CreatedAt: time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	return &AddEventAdminResponse{Success: true}, nil
}

//encore:api public method=POST path=/api/event-admins
func AddEventAdmin(ctx context.Context, p *AddEventAdminRequest) (*AddEventAdminResponse, error) {
	return AddEventAdminCore(ctx, p)
}

// RemoveEventAdminRequest carries the event/user ids plus the auth header
// (DELETE decodes the struct from query params).
//
// Mirrors ts-legacy/eventmanager/events.ts removeEventAdmin.
type RemoveEventAdminRequest struct {
	EventID       string `query:"eventId"`
	UserID        string `query:"userId"`
	Authorization string `header:"Authorization"`
}

// RemoveEventAdminCore removes an administrator from an event.
func RemoveEventAdminCore(ctx context.Context, p *RemoveEventAdminRequest) (*AddEventAdminResponse, error) {
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	if err := q().DeleteEventAdmin(ctx, sqlc.DeleteEventAdminParams{
		EventId: p.EventID,
		UserId:  p.UserID,
	}); err != nil {
		return nil, err
	}
	return &AddEventAdminResponse{Success: true}, nil
}

//encore:api public method=DELETE path=/api/event-admins
func RemoveEventAdmin(ctx context.Context, p *RemoveEventAdminRequest) (*AddEventAdminResponse, error) {
	return RemoveEventAdminCore(ctx, p)
}

// --- Delete ---

// DeleteEventRequest carries the event id plus the auth header (DELETE
// decodes the struct from query params).
type DeleteEventRequest struct {
	ID            string `query:"id"`
	Authorization string `header:"Authorization"`
	Permanent     bool   `query:"permanent"`
}

// DeleteEventResponse reports deletion.
type DeleteEventResponse struct {
	Deleted bool `json:"deleted"`
}

// DeleteEventCore soft-deletes an event (putting it into PENDING_DELETION) or permanently deletes it if specified/already pending.
func DeleteEventCore(ctx context.Context, p *DeleteEventRequest) (*DeleteEventResponse, error) {
	status, err := q().GetEventStatus(ctx, p.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "event not found"}
	}
	if err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.ID, auth.ActionDelete); err != nil {
		return nil, err
	}
	if p.Permanent || status == "PENDING_DELETION" {
		if err := q().DeleteEventByID(ctx, p.ID); err != nil {
			return nil, err
		}
	} else {
		now := time.Now().UTC()
		if err := q().SoftDeleteEvent(ctx, sqlc.SoftDeleteEventParams{
			DeletedAt: sql.NullTime{Time: now, Valid: true}, ID: p.ID,
		}); err != nil {
			return nil, err
		}
	}
	return &DeleteEventResponse{Deleted: true}, nil
}

//encore:api public method=DELETE path=/api/events
func DeleteEvent(ctx context.Context, p *DeleteEventRequest) (*DeleteEventResponse, error) {
	return DeleteEventCore(ctx, p)
}

// RestoreEventRequest carries the event id plus the auth header.
type RestoreEventRequest struct {
	ID            string `json:"id"`
	Authorization string `header:"Authorization"`
}

// RestoreEventCore restores a soft-deleted event back to PENDING.
func RestoreEventCore(ctx context.Context, p *RestoreEventRequest) (*EventDetail, error) {
	exists, err := q().EventExists(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, &errs.Error{Code: errs.NotFound, Message: "event not found"}
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.ID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	if err := q().RestoreEventStatus(ctx, p.ID); err != nil {
		return nil, err
	}
	return LoadEvent(ctx, p.ID)
}

//encore:api public method=POST path=/api/event-restore
func RestoreEvent(ctx context.Context, p *RestoreEventRequest) (*EventDetail, error) {
	return RestoreEventCore(ctx, p)
}
