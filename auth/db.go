package auth

import (
	"database/sql"
	"fmt"

	"encore.dev/storage/sqldb"

	"encore.app/auth/sqlc"
	"encore.app/shared"
)

// db is this service's handle on the shared "lightwing" database.
//
// The database resource itself (plus its migrations) is defined in the
// shared package, but Encore only grants a service access to a database the
// service references directly. Every query in this package goes through this
// handle (declared via sqldb.Named so Encore's static analysis wires up the
// access), never through shared.DB.
var db = sqldb.Named("lightwing")

func init() {
	shared.RegisterDB(&db)
}

// q returns the sqlc queries bound to the current database handle.
// db.Stdlib() tracks the active handle (including test swaps), so no
// cached pool or reset helper is needed here.
func q() *sqlc.Queries {
	return sqlc.New(db.Stdlib())
}

// nullStringFromAny maps a sqlc interface{} column (used for Postgres
// enums sqlc cannot resolve) back to sql.NullString, preserving NULL.
func nullStringFromAny(v any) sql.NullString {
	switch t := v.(type) {
	case nil:
		return sql.NullString{}
	case string:
		return sql.NullString{String: t, Valid: true}
	case []byte:
		return sql.NullString{String: string(t), Valid: true}
	default:
		return sql.NullString{String: fmt.Sprint(t), Valid: true}
	}
}
