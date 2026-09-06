package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"encore.dev/storage/sqldb"

	"encore.app/auth/sqlc"
)

// slugify normalizes a string into a URL-friendly slug (lowercase, hyphen-separated).
//
// Mirrors ts-legacy/lib/slugs.ts slugify:
//   toLowerCase().normalize("NFD").replace(/[\u0300-\u036f]/g, "")
//   .replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "")
func slugify(name string) string {
	s := strings.ToLower(name)
	s = normalizeNFD(s)
	s = replaceNonAlnum(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}

// normalizeNFD strips combining marks and decomposes precomposed accented
// characters — equivalent to the JS .normalize("NFD").replace(/[\u0300-\u036f]/g, "").
// This handles the common Latin-1 accented characters used in Discord display names.
func normalizeNFD(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	for _, r := range s {
		decomp := decomposeRune(r)
		if len(decomp) == 1 {
			// Single rune — check if it's a combining mark
			if r >= 0x0300 && r <= 0x036F {
				continue
			}
			b.WriteRune(r)
		} else {
			// Decomposed to multiple runes — write only the base (first rune)
			b.WriteRune(decomp[0])
		}
	}
	return b.String()
}

// decomposeRune decomposes a precomposed character into its NFD components.
// Returns the rune itself (as a single-element slice) if no decomposition exists.
// Handles Latin-1 Supplement and Latin Extended-A characters commonly found in names.
func decomposeRune(r rune) []rune {
	// Latin-1 Supplement (U+00C0–U+017F) — map to base ASCII + combining mark
	switch r {
	case 'á', 'à', 'â', 'ã', 'ä', 'å', 'ā', 'ă', 'ą':
		return []rune{'a', 0x0301} // a + acute
	case 'Á', 'À', 'Â', 'Ã', 'Ä', 'Å', 'Ā', 'Ă', 'Ą':
		return []rune{'A', 0x0301}
	case 'é', 'è', 'ê', 'ë', 'ē', 'ĕ', 'ę', 'ě':
		return []rune{'e', 0x0301}
	case 'É', 'È', 'Ê', 'Ë', 'Ē', 'Ĕ', 'Ę', 'Ě':
		return []rune{'E', 0x0301}
	case 'í', 'ì', 'î', 'ï', 'ī', 'ĭ', 'į', 'ı':
		return []rune{'i', 0x0301}
	case 'Í', 'Ì', 'Î', 'Ï', 'Ī', 'Ĭ', 'Į', 'I':
		return []rune{'I', 0x0301}
	case 'ó', 'ò', 'ô', 'õ', 'ö', 'ø', 'ō', 'ő', 'œ':
		return []rune{'o', 0x0301}
	case 'Ó', 'Ò', 'Ô', 'Õ', 'Ö', 'Ø', 'Ō', 'Ő', 'Œ':
		return []rune{'O', 0x0301}
	case 'ú', 'ù', 'û', 'ü', 'ũ', 'ū', 'ŭ', 'ů', 'ű':
		return []rune{'u', 0x0301}
	case 'Ú', 'Ù', 'Û', 'Ü', 'Ũ', 'Ū', 'Ŭ', 'Ů', 'Ű':
		return []rune{'U', 0x0301}
	case 'ç', 'ć', 'ĉ', 'ċ', 'č':
		return []rune{'c', 0x0301}
	case 'Ç', 'Ć', 'Ĉ', 'Ċ', 'Č':
		return []rune{'C', 0x0301}
	case 'ñ', 'ń', 'ņ', 'ň', 'ŋ':
		return []rune{'n', 0x0301}
	case 'Ñ', 'Ń', 'Ņ', 'Ň', 'Ŋ':
		return []rune{'N', 0x0301}
	case 'ý', 'ÿ', 'ŷ', 'ȳ':
		return []rune{'y', 0x0301}
	case 'Ý', 'Ŷ':
		return []rune{'Y', 0x0301}
	default:
		return []rune{r}
	}
}

func replaceNonAlnum(s, rep string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteString(rep)
		}
	}
	return b.String()
}

// toLowerAlnum converts to lowercase and strips non-alphanumeric chars.
func toLowerAlnum(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + 32)
		} else if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// truncate returns s shortened to at most max bytes.
// A non-positive max yields "". Slugs are ASCII-only, so byte slicing is safe.
func truncate(s string, max int) string {
	if max < 0 {
		max = 0
	}
	if len(s) > max {
		s = s[:max]
	}
	return s
}

// GenerateUniqueUserSlug generates a unique user slug starting from a base name.
// For users, slugs must be alphanumeric and 4-24 characters.
// If the base name doesn't produce a valid slug, falls back to a derivative
// of the user's Discord account ID.
//
// Mirrors ts-legacy/lib/slugs.ts generateUniqueUserSlug
func GenerateUniqueUserSlug(ctx context.Context, db *sql.DB, baseName string, userId string) (string, error) {
	qq := sqlc.New(db)
	// Normalize name to lowercase alphanumeric
	base := toLowerAlnum(baseName)

	// If base length exceeds 24 characters, truncate it
	if len(base) > 24 {
		base = base[:24]
	}

	// Check if alphanumeric and between 4 and 24 characters, and not reserved
	isValid := len(base) >= 4 && len(base) <= 24 && !isReservedSlug(base)

	slug := base
	if isValid {
		// Check collision
		existingId, err := qq.GetUserBySlug(ctx, sql.NullString{String: slug, Valid: true})
		if err == nil {
			if existingId == userId {
				return slug, nil
			}
		} else if errors.Is(err, sql.ErrNoRows) {
			// No existing user with this slug — we can use it
			return slug, nil
		} else {
			return "", fmt.Errorf("failed to check slug collision: %w", err)
		}

		// Collision! We must resolve it within 24 characters limit.
		counter := 2
		for {
			suffix := fmt.Sprintf("%d", counter)
			tempSlug := truncate(base, 24-len(suffix)) + suffix
			existingId, err := qq.GetUserBySlug(ctx, sql.NullString{String: tempSlug, Valid: true})
			if errors.Is(err, sql.ErrNoRows) || (err == nil && existingId == userId) {
				return tempSlug, nil
			}
			if err != nil {
				return "", fmt.Errorf("failed to check slug collision: %w", err)
			}
			counter++
		}
	}

	// Default to using a derivative of their Discord ID
	discordId, err := qq.GetDiscordAccountID(ctx, userId)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("failed to fetch discord account: %w", err)
	}
	if discordId == "" {
		discordId = userId
	}
	normalizedDiscordId := toLowerAlnum(discordId)
	// To make sure u + suffix is unique and matches u + discord ID/userId, take the rightmost characters
	lastPart := normalizedDiscordId
	if len(normalizedDiscordId) > 23 {
		lastPart = normalizedDiscordId[len(normalizedDiscordId)-23:]
	}
	slug = "u" + lastPart
	if len(slug) > 24 {
		slug = slug[:24]
	}

	counter := 2
	for {
		existingId, err := qq.GetUserBySlug(ctx, sql.NullString{String: slug, Valid: true})
		if errors.Is(err, sql.ErrNoRows) || (err == nil && existingId == userId) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to check slug collision: %w", err)
		}
		suffix := fmt.Sprintf("%d", counter)
		slug = truncate("u"+lastPart, 24-len(suffix)) + suffix
		counter++
	}

	return slug, nil
}

// GenerateUniqueOrgSlug generates a unique organization slug starting from a base name.
//
// Mirrors ts-legacy/lib/slugs.ts generateUniqueOrgSlug
func GenerateUniqueOrgSlug(ctx context.Context, db *sql.DB, baseName string) (string, error) {
	qq := sqlc.New(db)
	base := slugify(baseName)
	if base == "" || len(base) < 3 {
		base = "team"
	}
	if len(base) > 24 {
		base = base[:24]
	}

	slug := base
	if isReservedSlug(slug) {
		slug = base[:minInt(22, len(base))] + "-1"
	}

	counter := 2
	for {
		_, err := qq.GetOrgBySlug(ctx, slug)
		if errors.Is(err, sql.ErrNoRows) && !isReservedSlug(slug) {
			return slug, nil
		}
		if err == nil {
			// collision — fall through to retry
		} else if !errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("failed to check slug collision: %w", err)
		}
		suffix := fmt.Sprintf("-%d", counter)
		slug = truncate(base, 24-len(suffix)) + suffix
		counter++
	}
}

// ensureUserSlug lazily generates and assigns a user slug if they don't have one.
// Called from auth hooks (session creation).
//
// Mirrors ts-legacy/auth/auth.ts ensureUserSlug
func ensureUserSlug(ctx context.Context, db *sqldb.Database, userId string) (string, error) {
	row, err := q().GetUserNameSlug(ctx, userId)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to fetch user: %w", err)
	}
	if row.Slug.Valid && row.Slug.String != "" {
		return row.Slug.String, nil
	}

	baseSource := row.Name
	if baseSource == "" {
		baseSource = "user"
	}

	stdlibDB := db.Stdlib()
	newSlug, err := GenerateUniqueUserSlug(ctx, stdlibDB, baseSource, userId)
	if err != nil {
		return "", err
	}

	err = q().UpdateUserSlug(ctx, sqlc.UpdateUserSlugParams{
		Slug: sql.NullString{String: newSlug, Valid: true},
		ID:   userId,
	})
	if err != nil {
		return "", fmt.Errorf("failed to update user slug: %w", err)
	}
	return newSlug, nil
}

// IsValidSlug validates whether a team slug matches regex and length rules (3-24),
// allows hyphens, and is not reserved.
//
// Mirrors ts-legacy/lib/slugs.ts isValidSlug
func IsValidSlug(slug string) bool {
	if len(slug) < 3 || len(slug) > 24 {
		return false
	}
	if !validSlugRe.MatchString(slug) {
		return false
	}
	return !isReservedSlug(slug)
}

// IsValidUserSlug validates whether a user slug is alphanumeric-only and
// 4-24 characters, and is not reserved.
//
// Mirrors ts-legacy/lib/slugs.ts isValidUserSlug
func IsValidUserSlug(slug string) bool {
	if len(slug) < 4 || len(slug) > 24 {
		return false
	}
	if !userSlugRe.MatchString(slug) {
		return false
	}
	return !isReservedSlug(slug)
}
