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

// --- Add member ---

// AddEventMemberRequest carries the event/user ids plus the auth header.
//
// Mirrors ts-legacy/eventmanager/event-members.ts AddMemberParams
// (POST /api/events/:id/members).
type AddEventMemberRequest struct {
	EventID       string `json:"eventId"`
	UserID        string `json:"userId"`
	Authorization string `header:"Authorization"`
}

// AddEventMemberCore registers a participant, enforcing the event's class
// restriction and seeding the scoring record. Idempotent for existing members.
func AddEventMemberCore(ctx context.Context, p *AddEventMemberRequest) (*EventDetail, error) {
	tier, err := q().GetUserClassTier(ctx, p.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "user not found"}
	} else if err != nil {
		return nil, err
	}
	userTier := nullStringFromAny(tier)

	event, err := requireEventRow(ctx, p.EventID)
	if err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	if !IsEligible(toClassTier(classTierPtr(userTier)), toClassTier(classTierPtr(event.ClassRestriction))) {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "participant class tier does not satisfy the event class restriction"}
	}

	memberExists, err := q().EventMemberExists(ctx, sqlc.EventMemberExistsParams{
		EventId: p.EventID,
		UserId:  p.UserID,
	})
	if err != nil {
		return nil, err
	}
	if !memberExists {
		if event.ParticipantLimit.Valid {
			currentCount64, err := q().GetEventMemberCount(ctx, p.EventID)
			if err != nil {
				return nil, err
			}
			currentCount := int(currentCount64)
			if currentCount >= int(event.ParticipantLimit.Int64) {
				return nil, &errs.Error{
					Code:    errs.FailedPrecondition,
					Message: "Event participant capacity has been reached",
					Details: detailsMap{
						"code": CodeEventParticipantLimitReached,
						"limit": int(event.ParticipantLimit.Int64), "currentCount": currentCount,
					},
				}
			}
		}
		if err := q().InsertEventMember(ctx, sqlc.InsertEventMemberParams{
			ID: newID(), EventId: p.EventID, UserId: p.UserID, CreatedAt: time.Now().UTC(),
		}); err != nil {
			return nil, err
		}
	}
	if err := EnsureEventStandingsRow(ctx, p.EventID, p.UserID, event.ScoringType); err != nil {
		return nil, err
	}
	return LoadEvent(ctx, p.EventID)
}

//encore:api public method=POST path=/api/event-members
func AddEventMember(ctx context.Context, p *AddEventMemberRequest) (*EventDetail, error) {
	return AddEventMemberCore(ctx, p)
}

// RemoveMemberFromEvent deletes a member plus all associated standings and
// race participation rows.
//
// Mirrors ts-legacy/eventmanager/event-members.ts removeMemberFromEventInternal.
func RemoveMemberFromEvent(ctx context.Context, eventID, userID string) error {
	if err := q().DeleteEventMembersByUser(ctx, sqlc.DeleteEventMembersByUserParams{
		EventId: eventID, UserId: userID,
	}); err != nil {
		return err
	}
	if err := q().DeleteEventPointsByUser(ctx, sqlc.DeleteEventPointsByUserParams{
		EventId: eventID, UserId: userID,
	}); err != nil {
		return err
	}
	if err := q().DeleteEventLadderByUser(ctx, sqlc.DeleteEventLadderByUserParams{
		EventId: eventID, UserId: userID,
	}); err != nil {
		return err
	}
	if err := q().DeleteRaceEventMemberByEvent(ctx, sqlc.DeleteRaceEventMemberByEventParams{
		UserId: userID, EventId: eventID,
	}); err != nil {
		return err
	}
	if err := q().DeleteRaceResultsByEvent(ctx, sqlc.DeleteRaceResultsByEventParams{
		UserId: userID, EventId: eventID,
	}); err != nil {
		return err
	}
	return nil
}

// --- Remove member ---

// RemoveEventMemberRequest carries the event/user ids plus the auth header
// (DELETE decodes the struct from query params).
//
// Mirrors ts-legacy/eventmanager/event-members.ts RemoveMemberParams
// (DELETE /api/events/:id/members/:userId).
type RemoveEventMemberRequest struct {
	EventID       string `query:"eventId"`
	UserID        string `query:"userId"`
	Authorization string `header:"Authorization"`
}

// RemoveEventMemberCore removes a participant from an event.
func RemoveEventMemberCore(ctx context.Context, p *RemoveEventMemberRequest) (*EventDetail, error) {
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
	if err := RemoveMemberFromEvent(ctx, p.EventID, p.UserID); err != nil {
		return nil, err
	}
	return LoadEvent(ctx, p.EventID)
}

//encore:api public method=DELETE path=/api/event-members
func RemoveEventMember(ctx context.Context, p *RemoveEventMemberRequest) (*EventDetail, error) {
	return RemoveEventMemberCore(ctx, p)
}

// --- Join (self-service) ---

// JoinEventRequest carries the event id plus the auth header.
//
// Mirrors ts-legacy/eventmanager/event-members.ts JoinEventParams
// (POST /api/events/:id/join).
type JoinEventRequest struct {
	EventID       string `json:"eventId"`
	Authorization string `header:"Authorization"`
}

// JoinEventCore lets an authenticated user join a PENDING or ONGOING
// event. No event permission required; class restriction, signup lock, and
// capacity are enforced.
func JoinEventCore(ctx context.Context, p *JoinEventRequest) (*EventDetail, error) {
	actor, err := auth.ResolveActor(ctx, p.Authorization)
	if err != nil {
		return nil, err
	}
	userID := actor.UserID

	tier, err := q().GetUserClassTier(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "user not found"}
	} else if err != nil {
		return nil, err
	}
	userTier := nullStringFromAny(tier)

	event, err := requireEventRow(ctx, p.EventID)
	if err != nil {
		return nil, err
	}
	if event.Status != "PENDING" && event.Status != "ONGOING" {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "event is not open for public signup (must be PENDING or ONGOING)"}
	}
	if event.SignupsLocked {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "signups are locked for this event"}
	}
	if !IsEligible(toClassTier(classTierPtr(userTier)), toClassTier(classTierPtr(event.ClassRestriction))) {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "participant class tier does not satisfy the event class restriction"}
	}

	memberExists, err := q().EventMemberExists(ctx, sqlc.EventMemberExistsParams{
		EventId: p.EventID,
		UserId:  userID,
	})
	if err != nil {
		return nil, err
	}
	if !memberExists {
		if event.ParticipantLimit.Valid {
			currentCount64, err := q().GetEventMemberCount(ctx, p.EventID)
			if err != nil {
				return nil, err
			}
			currentCount := int(currentCount64)
			if currentCount >= int(event.ParticipantLimit.Int64) {
				return nil, &errs.Error{
					Code:    errs.FailedPrecondition,
					Message: "Event participant capacity has been reached",
					Details: detailsMap{
						"code": CodeEventParticipantLimitReached,
						"limit": int(event.ParticipantLimit.Int64), "currentCount": currentCount,
					},
				}
			}
		}
		if err := q().InsertEventMember(ctx, sqlc.InsertEventMemberParams{
			ID: newID(), EventId: p.EventID, UserId: userID, CreatedAt: time.Now().UTC(),
		}); err != nil {
			return nil, err
		}
	}
	if err := EnsureEventStandingsRow(ctx, p.EventID, userID, event.ScoringType); err != nil {
		return nil, err
	}
	return LoadEvent(ctx, p.EventID)
}

//encore:api public method=POST path=/api/event-join
func JoinEvent(ctx context.Context, p *JoinEventRequest) (*EventDetail, error) {
	return JoinEventCore(ctx, p)
}

// --- Leave (self-service) ---

// LeaveEventRequest carries the event id plus the auth header (DELETE
// decodes the struct from query params).
//
// Mirrors ts-legacy/eventmanager/event-members.ts LeaveEventParams
// (DELETE /api/events/:id/join).
type LeaveEventRequest struct {
	EventID       string `query:"eventId"`
	Authorization string `header:"Authorization"`
}

// LeaveEventCore lets an authenticated user withdraw from an event.
func LeaveEventCore(ctx context.Context, p *LeaveEventRequest) (*EventDetail, error) {
	event, err := requireEventRow(ctx, p.EventID)
	if err != nil {
		return nil, err
	}
	if event.SignupsLocked {
		return nil, &errs.Error{Code: errs.FailedPrecondition, Message: "signups are locked for this event"}
	}
	actor, err := auth.ResolveActor(ctx, p.Authorization)
	if err != nil {
		return nil, err
	}
	if err := RemoveMemberFromEvent(ctx, p.EventID, actor.UserID); err != nil {
		return nil, err
	}
	return LoadEvent(ctx, p.EventID)
}

//encore:api public method=DELETE path=/api/event-join
func LeaveEvent(ctx context.Context, p *LeaveEventRequest) (*EventDetail, error) {
	return LeaveEventCore(ctx, p)
}

// --- Signups lock ---

// SetEventSignupsLockedRequest toggles the signup lock plus the auth header.
//
// Mirrors ts-legacy/eventmanager/event-members.ts SetSignupsLockedParams
// (PUT /api/events/:id/signups-lock).
type SetEventSignupsLockedRequest struct {
	EventID       string `json:"eventId"`
	Locked        bool   `json:"locked"`
	Authorization string `header:"Authorization"`
}

// SetEventSignupsLockedCore toggles an event's signup lock. Gated by
// event-update permission.
func SetEventSignupsLockedCore(ctx context.Context, p *SetEventSignupsLockedRequest) (*EventDetail, error) {
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
	if err := q().UpdateEventSignupsLocked(ctx, sqlc.UpdateEventSignupsLockedParams{
		SignupsLocked: p.Locked, UpdatedAt: time.Now().UTC(), ID: p.EventID,
	}); err != nil {
		return nil, err
	}
	return LoadEvent(ctx, p.EventID)
}

//encore:api public method=PUT path=/api/event-signups-lock
func SetEventSignupsLocked(ctx context.Context, p *SetEventSignupsLockedRequest) (*EventDetail, error) {
	return SetEventSignupsLockedCore(ctx, p)
}
