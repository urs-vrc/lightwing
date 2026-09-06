package eventmanager

import (
	"context"
	"database/sql"
	"time"

	"encore.dev/beta/errs"
	"encore.app/auth"
	"encore.app/eventmanager/sqlc"
)

// AddEventScheduleRequest carries the schedule payload plus the auth header.
//
// Mirrors ts-legacy/eventmanager/event-schedules.ts AddScheduleParams
// (POST /api/events/:id/schedules).
type AddEventScheduleRequest struct {
	EventID       string  `json:"eventId"`
	Title         *string `json:"title,omitempty"`
	StartsAt      string  `json:"startsAt"`
	EndsAt        *string `json:"endsAt,omitempty"`
	Location      *string `json:"location,omitempty"`
	Authorization string  `header:"Authorization"`
}

// AddEventScheduleCore adds a schedule slot to an event.
func AddEventScheduleCore(ctx context.Context, p *AddEventScheduleRequest) (*EventDetail, error) {
	exists, err := q().EventExists(ctx, p.EventID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, &errs.Error{Code: errs.NotFound, Message: "event not found"}
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}

	startsAt, err := time.Parse(time.RFC3339Nano, p.StartsAt)
	if err != nil {
		return nil, &errs.Error{Code: errs.InvalidArgument, Message: "startsAt must be an ISO-8601 timestamp"}
	}
	var endsAt *time.Time
	if p.EndsAt != nil && *p.EndsAt != "" {
		t, err := time.Parse(time.RFC3339Nano, *p.EndsAt)
		if err != nil {
			return nil, &errs.Error{Code: errs.InvalidArgument, Message: "endsAt must be an ISO-8601 timestamp"}
		}
		utc := t.UTC()
		endsAt = &utc
	}

	var title, location sql.NullString
	if p.Title != nil {
		title = sql.NullString{String: *p.Title, Valid: true}
	}
	var endsAtNull sql.NullTime
	if endsAt != nil {
		endsAtNull = sql.NullTime{Time: *endsAt, Valid: true}
	}
	if p.Location != nil {
		location = sql.NullString{String: *p.Location, Valid: true}
	}
	if err := q().InsertEventSchedule(ctx, sqlc.InsertEventScheduleParams{
		ID: newID(), EventId: p.EventID, Title: title, StartsAt: startsAt.UTC(),
		EndsAt: endsAtNull, Location: location, CreatedAt: time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	return LoadEvent(ctx, p.EventID)
}

//encore:api public method=POST path=/api/event-schedules
func AddEventSchedule(ctx context.Context, p *AddEventScheduleRequest) (*EventDetail, error) {
	return AddEventScheduleCore(ctx, p)
}
