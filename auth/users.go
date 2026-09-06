package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"encore.dev/beta/errs"

	"encore.app/auth/sqlc"
)

// TeamAffiliation represents a user's membership in an organization.
//
// Mirrors ts-legacy/auth/users.ts TeamAffiliation
type TeamAffiliation struct {
	OrganizationID   string `json:"organizationId"`
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	Role             string `json:"role"`
}

// UserProfile is the public representation of a user, as returned by
// get-session and the user management endpoints.
//
// Mirrors ts-legacy/auth/users.ts UserProfile. Nullable columns are
// pointers so they serialize as null when unset, and Name prefers the
// VRChat display name exactly like TS toProfile does.
type UserProfile struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Slug           *string           `json:"slug"`
	Email          string            `json:"email"`
	Image          *string           `json:"image"`
	Biography      *string           `json:"biography"`
	CareerOverview *string           `json:"careerOverview"`
	VrchatUsername *string           `json:"vrchatUsername"`
	ClassTier      *string           `json:"classTier"`
	SiteRole       string            `json:"siteRole"`
	Teams          []TeamAffiliation `json:"teams"`
	CreatedAt      string            `json:"createdAt"`
	UpdatedAt      string            `json:"updatedAt"`
}

// userRow is the raw user record plus team affiliations.
type userRow struct {
	ID             string
	Name           string
	Email          string
	Image          sql.NullString
	Slug           sql.NullString
	Biography      sql.NullString
	CareerOverview sql.NullString
	VrchatUsername sql.NullString
	ClassTier      sql.NullString
	SiteRole       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func nullToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

func loadUserRow(ctx context.Context, userID string) (*userRow, error) {
	r, err := q().GetUserProfileRow(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "user not found"}
	}
	if err != nil {
		return nil, err
	}
	return &userRow{
		ID: r.ID, Name: r.Name, Email: r.Email, Image: r.Image,
		Slug: r.Slug, Biography: r.Biography, CareerOverview: r.CareerOverview,
		VrchatUsername: r.VrchatUsername, ClassTier: nullStringFromAny(r.ClassTier),
		SiteRole: r.SiteRole, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, nil
}

func loadTeams(ctx context.Context, userID string) ([]TeamAffiliation, error) {
	teamsByUser, err := loadTeamsForUsers(ctx, []string{userID})
	if err != nil {
		return nil, err
	}
	teams := teamsByUser[userID]
	if teams == nil {
		return []TeamAffiliation{}, nil
	}
	return teams, nil
}

func loadTeamsForUsers(ctx context.Context, userIDs []string) (map[string][]TeamAffiliation, error) {
	if len(userIDs) == 0 {
		return map[string][]TeamAffiliation{}, nil
	}
	affils, err := q().ListTeamAffiliations(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query team affiliations: %w", err)
	}

	teamsByUser := make(map[string][]TeamAffiliation)
	for _, a := range affils {
		teamsByUser[a.UserId] = append(teamsByUser[a.UserId], TeamAffiliation{
			OrganizationID: a.ID,
			Name:           a.Name,
			Slug:           a.Slug,
			Role:           a.Role,
		})
	}
	return teamsByUser, nil
}

// toProfile maps a user row plus affiliations to the public profile.
// The display name prefers the VRChat username, mirroring TS toProfile.
func (r *userRow) toProfile(teams []TeamAffiliation) *UserProfile {
	name := r.Name
	if r.VrchatUsername.Valid && r.VrchatUsername.String != "" {
		name = r.VrchatUsername.String
	}
	return &UserProfile{
		ID:             r.ID,
		Name:           name,
		Slug:           nullToPtr(r.Slug),
		Email:          r.Email,
		Image:          nullToPtr(r.Image),
		Biography:      nullToPtr(r.Biography),
		CareerOverview: nullToPtr(r.CareerOverview),
		VrchatUsername: nullToPtr(r.VrchatUsername),
		ClassTier:      nullToPtr(r.ClassTier),
		SiteRole:       r.SiteRole,
		Teams:          teams,
		CreatedAt:      r.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:      r.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func loadUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	row, err := loadUserRow(ctx, userID)
	if err != nil {
		return nil, err
	}
	teams, err := loadTeams(ctx, userID)
	if err != nil {
		return nil, err
	}
	return row.toProfile(teams), nil
}

// getUserProfile fetches the full public profile for a user by ID.
//
// Mirrors ts-legacy/auth/users.ts getUserProfile
func getUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	return loadUserProfile(ctx, userID)
}

// getUserProfileBySlug fetches a user profile by slug.
//
// Mirrors ts-legacy/auth/users.ts getUserProfileBySlug
func getUserProfileBySlug(ctx context.Context, slug string) (*UserProfile, error) {
	userID, err := q().GetUserBySlug(ctx, sql.NullString{String: slug, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, &errs.Error{Code: errs.NotFound, Message: "user not found"}
	}
	if err != nil {
		return nil, err
	}
	return loadUserProfile(ctx, userID)
}

// UpdateUserProfileParams are the editable profile fields. A nil pointer
// leaves the field untouched; a non-nil pointer sets it (mirroring TS,
// where only provided keys are written).
type UpdateUserProfileParams struct {
	Name           *string `json:"name,omitempty"`
	Slug           *string `json:"slug,omitempty"`
	Image          *string `json:"image,omitempty"`
	Biography      *string `json:"biography,omitempty"`
	CareerOverview *string `json:"careerOverview,omitempty"`
	VrchatUsername *string `json:"vrchatUsername,omitempty"`
}

// updateUserProfile updates a user's editable profile fields. A user may
// only edit their own record; site admins may edit any profile.
//
// Mirrors ts-legacy/auth/users.ts updateUserProfile
func updateUserProfile(ctx context.Context, actor *Actor, targetUserID string, params *UpdateUserProfileParams) (*UserProfile, error) {
	if actor.UserID != targetUserID && !isSiteAdmin(actor.SiteRole) {
		return nil, &errs.Error{Code: errs.PermissionDenied, Message: "cannot edit another user's profile"}
	}

	existing, err := loadUserRow(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	up := sqlc.UpdateUserProfileParams{UpdatedAt: time.Now().UTC(), ID: targetUserID}

	if params.Name != nil {
		up.Name = sql.NullString{String: *params.Name, Valid: true}
	}
	if params.Slug != nil && (existing.Slug.String != *params.Slug || !existing.Slug.Valid) {
		if !IsValidUserSlug(*params.Slug) {
			return nil, &errs.Error{Code: errs.InvalidArgument, Message: "invalid slug format or length"}
		}
		_, err := q().GetUserBySlug(ctx, sql.NullString{String: *params.Slug, Valid: true})
		if err == nil {
			return nil, &errs.Error{Code: errs.AlreadyExists, Message: "user slug is already in use"}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to check slug collision: %w", err)
		}
		up.Slug = sql.NullString{String: *params.Slug, Valid: true}
	}
	if params.Image != nil {
		up.ImageSet = true
		up.ImageVal = nullIfEmpty(*params.Image)
	}
	if params.Biography != nil {
		up.BiographySet = true
		up.BiographyVal = nullIfEmpty(*params.Biography)
	}
	if params.CareerOverview != nil {
		up.CareerSet = true
		up.CareerVal = nullIfEmpty(*params.CareerOverview)
	}
	if params.VrchatUsername != nil {
		up.VrchatSet = true
		up.VrchatVal = nullIfEmpty(*params.VrchatUsername)
	}
	if err := q().UpdateUserProfile(ctx, up); err != nil {
		return nil, fmt.Errorf("failed to update user profile: %w", err)
	}
	return loadUserProfile(ctx, targetUserID)
}

// nullIfEmpty maps "" to NULL so clearing a field stores NULL like the
// TS Prisma update with null does.
func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// SetUserSiteRoleParams provides the input for assigning a site role.
type SetUserSiteRoleParams struct {
	UserID   string `json:"userId"`
	SiteRole string `json:"siteRole"`
}

// setUserSiteRole assigns a global site role to a user and returns the
// updated profile. Restricted to existing site administrators.
//
// Mirrors ts-legacy/auth/users.ts setUserSiteRole
func setUserSiteRole(ctx context.Context, actor *Actor, params *SetUserSiteRoleParams) (*UserProfile, error) {
	if !isSiteAdmin(actor.SiteRole) {
		return nil, &errs.Error{Code: errs.PermissionDenied, Message: "site admin required"}
	}
	if params.SiteRole != string(SiteRoleUser) && params.SiteRole != string(SiteRoleSiteAdmin) {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: fmt.Sprintf("invalid site role: %s", params.SiteRole),
		}
	}
	if _, err := loadUserRow(ctx, params.UserID); err != nil {
		return nil, err
	}
	if err := q().UpdateUserSiteRole(ctx, sqlc.UpdateUserSiteRoleParams{
		SiteRole: params.SiteRole,
		ID:       params.UserID,
	}); err != nil {
		return nil, fmt.Errorf("failed to set user site role: %w", err)
	}
	return loadUserProfile(ctx, params.UserID)
}

// ListUsersQuery filters for listUsers. Limit <= 0 means no limit.
type ListUsersQuery struct {
	Search string
	Limit  int
	Offset int
}

// ListUsersResponse is the paginated response for listUsers.
type ListUsersResponse struct {
	Users []UserProfile `json:"users"`
	Total int            `json:"total"`
}

// likePattern escapes LIKE wildcards so search behaves as a literal
// case-insensitive contains, mirroring Prisma's contains mode.
func likePattern(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}

// listUsers returns paginated user profiles with optional case-insensitive
// search across name, VRChat username, email, and slug. Gated on the caller
// being a site administrator.
//
// Mirrors ts-legacy/auth/users.ts listUsers
func listUsers(ctx context.Context, actor *Actor, q ListUsersQuery) (*ListUsersResponse, error) {
	if !isSiteAdmin(actor.SiteRole) {
		return nil, &errs.Error{Code: errs.PermissionDenied, Message: "site admin required"}
	}

	pattern := ""
	if q.Search != "" {
		pattern = likePattern(q.Search)
	}

	total64, err := sqlc.New(db.Stdlib()).CountUsersBySearch(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}
	total := int(total64)

	urows, err := sqlc.New(db.Stdlib()).ListUserRows(ctx, sqlc.ListUserRowsParams{
		Column1: pattern,
		Column2: int32(q.Limit),
		Column3: int32(q.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}

	userRows := []userRow{}
	userIDs := []string{}
	for _, ur := range urows {
		r := userRow{
			ID: ur.ID, Name: ur.Name, Email: ur.Email, Image: ur.Image,
			Slug: ur.Slug, Biography: ur.Biography, CareerOverview: ur.CareerOverview,
			VrchatUsername: ur.VrchatUsername, ClassTier: nullStringFromAny(ur.ClassTier),
			SiteRole: ur.SiteRole, CreatedAt: ur.CreatedAt, UpdatedAt: ur.UpdatedAt,
		}
		userRows = append(userRows, r)
		userIDs = append(userIDs, r.ID)
	}

	teamsByUser, err := loadTeamsForUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	resp := &ListUsersResponse{Users: make([]UserProfile, 0, len(userRows)), Total: total}
	for _, r := range userRows {
		teams := teamsByUser[r.ID]
		if teams == nil {
			teams = []TeamAffiliation{}
		}
		resp.Users = append(resp.Users, *r.toProfile(teams))
	}
	return resp, nil
}

// getUserTeamAffiliations returns a user's organization memberships.
func getUserTeamAffiliations(ctx context.Context, userID string) ([]TeamAffiliation, error) {
	return loadTeams(ctx, userID)
}

// --- HTTP endpoints (thin wrappers over the internals above) ---

// GetUser returns a participant's public profile.
//
// Mirrors ts-legacy/auth/users.ts getUserProfile (GET /api/users/:id)
//
//encore:api public method=GET path=/api/users/:id
func (s *Service) GetUser(ctx context.Context, id string) (*UserProfile, error) {
	return getUserProfile(ctx, id)
}

// GetUserBySlug returns a participant's public profile by slug.
//
// Mirrors ts-legacy/auth/users.ts getUserProfileBySlug. It lives at
// /api/users-by-slug (rather than nested under /api/users/) because static
// sub-paths route-conflict with /api/users/:id under Encore's router.
//
//encore:api public method=GET path=/api/users-by-slug
func (s *Service) GetUserBySlug(ctx context.Context, p *GetUserBySlugParams) (*UserProfile, error) {
	return getUserProfileBySlug(ctx, p.Slug)
}

// GetUserBySlugParams carries the slug query param.
type GetUserBySlugParams struct {
	Slug string `query:"slug"`
}

// UpdateUserRequest carries the target id, auth header, and editable fields.
// (The id travels in the body rather than a :id path param because this
// Encore version only accepts scalar params alongside path params.)
//
// Mirrors ts-legacy/auth/users.ts updateUserProfile (PATCH /api/users/:id)
type UpdateUserRequest struct {
	ID              string  `json:"id"`
	Authorization   string  `header:"Authorization"`
	Name            *string `json:"name,omitempty"`
	Slug            *string `json:"slug,omitempty"`
	Image           *string `json:"image,omitempty"`
	Biography       *string `json:"biography,omitempty"`
	CareerOverview  *string `json:"careerOverview,omitempty"`
	VrchatUsername  *string `json:"vrchatUsername,omitempty"`
}

//encore:api public method=PATCH path=/api/users
func (s *Service) UpdateUser(ctx context.Context, p *UpdateUserRequest) (*UserProfile, error) {
	actor, err := resolveActor(ctx, p.Authorization)
	if err != nil {
		return nil, err
	}
	return updateUserProfile(ctx, actor, p.ID, &UpdateUserProfileParams{
		Name:           p.Name,
		Slug:           p.Slug,
		Image:          p.Image,
		Biography:      p.Biography,
		CareerOverview: p.CareerOverview,
		VrchatUsername: p.VrchatUsername,
	})
}

// SetUserSiteRoleRequest carries the target id, auth header, and new role.
// (The id travels in the body rather than a :id path param because this
// Encore version only accepts scalar params alongside path params.)
//
// Mirrors ts-legacy/auth/users.ts setUserSiteRole
// (PUT /api/users/:id/site-role)
type SetUserSiteRoleRequest struct {
	ID            string `json:"id"`
	Authorization string `header:"Authorization"`
	SiteRole      string `json:"siteRole"`
}

//encore:api public method=PUT path=/api/users/site-role
func (s *Service) SetUserSiteRole(ctx context.Context, p *SetUserSiteRoleRequest) (*UserProfile, error) {
	actor, err := resolveActor(ctx, p.Authorization)
	if err != nil {
		return nil, err
	}
	return setUserSiteRole(ctx, actor, &SetUserSiteRoleParams{UserID: p.ID, SiteRole: p.SiteRole})
}

// ListUsersRequest carries the auth header plus search/pagination query params.
//
// Mirrors ts-legacy/auth/users.ts listUsers (GET /api/users)
type ListUsersRequest struct {
	Authorization string `header:"Authorization"`
	Search        string `query:"search"`
	Limit         int    `query:"limit"`
	Offset        int    `query:"offset"`
}

//encore:api public method=GET path=/api/users
func (s *Service) ListUsers(ctx context.Context, p *ListUsersRequest) (*ListUsersResponse, error) {
	actor, err := resolveActor(ctx, p.Authorization)
	if err != nil {
		return nil, err
	}
	return listUsers(ctx, actor, ListUsersQuery{Search: p.Search, Limit: p.Limit, Offset: p.Offset})
}
