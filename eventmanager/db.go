package eventmanager

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"encore.dev/storage/sqldb"

	"encore.app/eventmanager/sqlc"
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

var (
	stdMu   sync.Mutex
	stdPool *sql.DB
)

// std returns a database/sql pool over the current handle, rebuilding it
// lazily. Libraries like sqlc need this bridge; raw queries keep using db.
func std() *sql.DB {
	stdMu.Lock()
	defer stdMu.Unlock()
	if stdPool == nil {
		connStr := sqldb.RegisterStdlibDriver(db)
		pool, err := sql.Open("encore", connStr)
		if err != nil {
			panic("eventmanager: open stdlib db: " + err.Error())
		}
		stdPool = pool
	}
	return stdPool
}

// resetStdPool drops the cached pool so the next q() rebuilds it from the
// current handle. Called from TestMain after shared.SetTestDB.
func resetStdPool() {
	stdMu.Lock()
	defer stdMu.Unlock()
	if stdPool != nil {
		_ = stdPool.Close()
		stdPool = nil
	}
}

// q returns the sqlc queries bound to the current database handle.
func q() *sqlc.Queries {
	return sqlc.New(std())
}

// nullStringFromPtr maps an optional *string to sql.NullString,
// preserving nil as NULL.
func nullStringFromPtr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// nullTimeFromPtr maps an optional *time.Time to sql.NullTime,
// preserving nil as NULL.
func nullTimeFromPtr(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// timePtrFromNull maps sql.NullTime to *time.Time, preserving NULL as nil.
func timePtrFromNull(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	t := nt.Time
	return &t
}

// stringFromAny maps a sqlc interface{} column (used for Postgres enums
// sqlc cannot resolve) to string. Callers must only use it on NOT NULL
// columns; nullable ones go through nullStringFromAny.
func stringFromAny(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
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
