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

// Class tiers and datasets endpoints.
//
// Mirrors ts-legacy/eventmanager/classes.ts and datasets.ts.

// ClassTierInfo describes one skill tier.
type ClassTierInfo struct {
	Tier  string `json:"tier"`
	Label string `json:"label"`
	Rank  int    `json:"rank"`
}

// ClassTiersResponse wraps the ordered tier list.
type ClassTiersResponse struct {
	Tiers []ClassTierInfo `json:"tiers"`
}

//encore:api public method=GET path=/api/classes
func ListClassTiers(ctx context.Context) (*ClassTiersResponse, error) {
	resp := &ClassTiersResponse{}
	for i, tier := range ClassTierOrder {
		resp.Tiers = append(resp.Tiers, ClassTierInfo{
			Tier:  string(tier),
			Label: ClassTierLabels[tier],
			Rank:  i + 1,
		})
	}
	return resp, nil
}

// SetUserClassRequest mirrors SetUserClassParams (PUT /api/users/:userId/class).
// Within an organization it is gated by the event-update grant; site admins
// may set a tier globally.
type SetUserClassRequest struct {
	UserID         string  `json:"userId"`
	Authorization  string  `header:"Authorization"`
	OrganizationID *string `json:"organizationId,omitempty"`
	ClassTier      *string `json:"classTier"`
}

// SetUserClassResponse echoes the assignment.
type SetUserClassResponse struct {
	UserID    string  `json:"userId"`
	ClassTier *string `json:"classTier"`
}

// SetUserClassCore tags a participant with a skill class tier.
func SetUserClassCore(ctx context.Context, p *SetUserClassRequest) (*SetUserClassResponse, error) {
	if p.OrganizationID != nil && *p.OrganizationID != "" {
		if _, _, err := auth.RequirePermission(ctx, p.Authorization, *p.OrganizationID, auth.ResourceEvent, auth.ActionUpdate); err != nil {
			return nil, err
		}
	} else if _, err := auth.RequireSiteAdmin(ctx, p.Authorization); err != nil {
		return nil, err
	}
	exists, err := q().UserExists(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, &errs.Error{Code: errs.NotFound, Message: "user not found"}
	}
	var tier any
	if p.ClassTier != nil {
		tier = *p.ClassTier
	}
	if err := q().UpdateUserClassTier(ctx, sqlc.UpdateUserClassTierParams{
		ClassTier: tier, ID: p.UserID,
	}); err != nil {
		return nil, err
	}
	return &SetUserClassResponse{UserID: p.UserID, ClassTier: p.ClassTier}, nil
}

//encore:api public method=PUT path=/api/user-class
func SetUserClass(ctx context.Context, p *SetUserClassRequest) (*SetUserClassResponse, error) {
	return SetUserClassCore(ctx, p)
}

// EligibleRace is one race the participant may enter.
type EligibleRace struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Sequence         int     `json:"sequence"`
	ClassRestriction *string `json:"classRestriction"`
}

// EligibleEvent is one event with enterable races.
type EligibleEvent struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	OrganizationID   *string        `json:"organizationId"`
	ClassRestriction *string        `json:"classRestriction"`
	EligibleRaces    []EligibleRace `json:"eligibleRaces"`
}

// EligibleEventsQuery fetches events a participant may enter.
type EligibleEventsQuery struct {
	UserID string `query:"userId"`
}

// EligibleEventsResponse wraps the eligible event list.
type EligibleEventsResponse struct {
	Events []*EligibleEvent `json:"events"`
}

// ListEligibleEventsCore returns events the participant may enter based on
// class tier and each event's class restriction.
func ListEligibleEventsCore(ctx context.Context, q *EligibleEventsQuery) (*EligibleEventsResponse, error) {
	tierAny, err := sqlc.New(std()).GetUserClassTier(ctx, q.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "user not found"}
	}
	if err != nil {
		return nil, err
	}
	userTier := nullStringFromAny(tierAny)
	var tier *ClassTier
	if userTier.Valid {
		t := ClassTier(userTier.String)
		tier = &t
	}
	erows, err := sqlc.New(std()).ListEligibleEvents(ctx)
	if err != nil {
		return nil, err
	}
	type eventInfo struct {
		id           string
		name         string
		orgID        sql.NullString
		classRestr   sql.NullString
	}
	var events []eventInfo
	for _, er := range erows {
		events = append(events, eventInfo{
			id: er.ID, name: er.Name, orgID: er.OrganizationId,
			classRestr: nullStringFromAny(er.ClassRestriction),
		})
	}
	resp := &EligibleEventsResponse{Events: []*EligibleEvent{}}
	for _, e := range events {
		var eventRestr *ClassTier
		if e.classRestr.Valid {
			t := ClassTier(e.classRestr.String)
			eventRestr = &t
		}
		eventEligible := IsEligible(tier, eventRestr)
		rrows, err := sqlc.New(std()).ListEligibleRaces(ctx, e.id)
		if err != nil {
			return nil, err
		}
		eligibleRaces := []EligibleRace{}
		for _, rr := range rrows {
			rc := nullStringFromAny(rr.ClassRestriction)
			effective := eventRestr
			if rc.Valid {
				t := ClassTier(rc.String)
				effective = &t
			}
			if IsEligible(tier, effective) {
				var effStr *string
				if effective != nil {
					s := string(*effective)
					effStr = &s
				}
				eligibleRaces = append(eligibleRaces, EligibleRace{
					ID: rr.ID, Name: rr.Name, Sequence: int(rr.Sequence), ClassRestriction: effStr,
				})
			}
		}
		if eventEligible || len(eligibleRaces) > 0 {
			resp.Events = append(resp.Events, &EligibleEvent{
				ID:               e.id,
				Name:             e.name,
				OrganizationID:   nullString(e.orgID),
				ClassRestriction: nullString(e.classRestr),
				EligibleRaces:    eligibleRaces,
			})
		}
	}
	return resp, nil
}

//encore:api public method=GET path=/api/eligible-events
func ListEligibleEvents(ctx context.Context, q *EligibleEventsQuery) (*EligibleEventsResponse, error) {
	return ListEligibleEventsCore(ctx, q)
}

// --- Datasets ---

// DatasetView mirrors the TS DatasetView.
type DatasetView struct {
	ID         string  `json:"id"`
	EventID    string  `json:"eventId"`
	Source     string  `json:"source"`
	Rows       int     `json:"rows"`
	Status     string  `json:"status"`
	ImportedAt *string `json:"importedAt"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
}

type datasetRow struct {
	ID         string
	EventID    string
	Source     string
	Rows       int
	Status     string
	ImportedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func toDatasetView(d *datasetRow) *DatasetView {
	return &DatasetView{
		ID:         d.ID,
		EventID:    d.EventID,
		Source:     d.Source,
		Rows:       d.Rows,
		Status:     d.Status,
		ImportedAt: nullTime(d.ImportedAt),
		CreatedAt:  isoTime(d.CreatedAt),
		UpdatedAt:  isoTime(d.UpdatedAt),
	}
}

// DatasetsQuery scopes dataset listing by event.
type DatasetsQuery struct {
	EventID string `query:"eventId"`
}

// DatasetsResponse wraps the dataset list.
type DatasetsResponse struct {
	Datasets []*DatasetView `json:"datasets"`
}

// toDatasetRow maps a sqlc dataset row onto the local datasetRow.
func toDatasetRow(id, eventID, source string, rows int32, status any, importedAt sql.NullTime, createdAt, updatedAt time.Time) *datasetRow {
	return &datasetRow{
		ID: id, EventID: eventID, Source: source, Rows: int(rows),
		Status: stringFromAny(status), ImportedAt: timePtrFromNull(importedAt),
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
}

// ListDatasetsCore lists datasets scoped by event.
func ListDatasetsCore(ctx context.Context, q *DatasetsQuery) (*DatasetsResponse, error) {
	if _, err := requireEventRow(ctx, q.EventID); err != nil {
		return nil, err
	}
	drows, err := sqlc.New(std()).ListDatasetsByEvent(ctx, q.EventID)
	if err != nil {
		return nil, err
	}
	resp := &DatasetsResponse{Datasets: []*DatasetView{}}
	for _, dr := range drows {
		resp.Datasets = append(resp.Datasets, toDatasetView(toDatasetRow(
			dr.ID, dr.EventId, dr.Source, dr.Rows, dr.Status,
			dr.ImportedAt, dr.CreatedAt, dr.UpdatedAt)))
	}
	return resp, nil
}

//encore:api public method=GET path=/api/datasets-list
func ListDatasets(ctx context.Context, q *DatasetsQuery) (*DatasetsResponse, error) {
	return ListDatasetsCore(ctx, q)
}

// CreateDatasetRequest mirrors CreateDatasetParams.
type CreateDatasetRequest struct {
	EventID       string  `json:"eventId"`
	Authorization string  `header:"Authorization"`
	Source        string  `json:"source"`
	Rows          int     `json:"rows"`
	Status        *string `json:"status,omitempty"`
}

// CreateDatasetCore creates a dataset record for an event.
func CreateDatasetCore(ctx context.Context, p *CreateDatasetRequest) (*DatasetView, error) {
	if _, err := requireEventRow(ctx, p.EventID); err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	status := "PENDING"
	if p.Status != nil {
		status = *p.Status
	}
	var importedAt *time.Time
	if status == "DONE" {
		now := time.Now().UTC()
		importedAt = &now
	}
	id := "dataset-" + newID()[:8]
	now := time.Now().UTC()
	created, err := q().CreateDataset(ctx, sqlc.CreateDatasetParams{
		ID: id, EventId: p.EventID, Source: p.Source, Rows: int32(p.Rows),
		Status: status, ImportedAt: nullTimeFromPtr(importedAt), CreatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	return toDatasetView(toDatasetRow(
		created.ID, created.EventId, created.Source, created.Rows, created.Status,
		created.ImportedAt, created.CreatedAt, created.UpdatedAt)), nil
}

//encore:api public method=POST path=/api/datasets
func CreateDataset(ctx context.Context, p *CreateDatasetRequest) (*DatasetView, error) {
	return CreateDatasetCore(ctx, p)
}

// UpdateDatasetStatusRequest mirrors UpdateDatasetStatusParams.
type UpdateDatasetStatusRequest struct {
	EventID       string `json:"eventId"`
	DatasetID     string `json:"datasetId"`
	Authorization string `header:"Authorization"`
	Status        string `json:"status"`
}

// UpdateDatasetStatusCore updates a dataset record's processing status.
func UpdateDatasetStatusCore(ctx context.Context, p *UpdateDatasetStatusRequest) (*DatasetView, error) {
	if _, err := requireEventRow(ctx, p.EventID); err != nil {
		return nil, err
	}
	if _, err := auth.RequireEventPermission(ctx, p.Authorization, p.EventID, auth.ActionUpdate); err != nil {
		return nil, err
	}
	fetched, err := q().GetDatasetByID(ctx, sqlc.GetDatasetByIDParams{
		ID: p.DatasetID, EventId: p.EventID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "dataset not found"}
	}
	if err != nil {
		return nil, err
	}
	d := toDatasetRow(
		fetched.ID, fetched.EventId, fetched.Source, fetched.Rows, fetched.Status,
		fetched.ImportedAt, fetched.CreatedAt, fetched.UpdatedAt)
	importedAt := d.ImportedAt
	if p.Status == "DONE" {
		now := time.Now().UTC()
		importedAt = &now
	}
	updated, err := q().UpdateDatasetStatus(ctx, sqlc.UpdateDatasetStatusParams{
		Status: p.Status, ImportedAt: nullTimeFromPtr(importedAt), ID: p.DatasetID,
	})
	if err != nil {
		return nil, err
	}
	return toDatasetView(toDatasetRow(
		updated.ID, updated.EventId, updated.Source, updated.Rows, updated.Status,
		updated.ImportedAt, updated.CreatedAt, updated.UpdatedAt)), nil
}

//encore:api public method=PUT path=/api/dataset-status
func UpdateDatasetStatus(ctx context.Context, p *UpdateDatasetStatusRequest) (*DatasetView, error) {
	return UpdateDatasetStatusCore(ctx, p)
}
