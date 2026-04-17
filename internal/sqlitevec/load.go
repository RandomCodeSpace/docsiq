//go:build cgo

package sqlitevec

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	sqlite3 "github.com/mattn/go-sqlite3"
)

// ErrExtensionMissing is returned when the extension file does not exist
// on disk at LoadInto time. Callers should treat this as "fall back".
var ErrExtensionMissing = errors.New("sqlitevec: extension file missing on disk")

// ErrExtensionEmpty is returned when the extension file is 0 bytes (i.e.
// the embedded asset was a placeholder that got extracted, so the path
// exists but the file is useless). Callers should treat this as "fall
// back".
var ErrExtensionEmpty = errors.New("sqlitevec: extension file is empty (placeholder)")

// LoadInto loads the sqlite-vec extension at soPath into every connection
// of db. Implementation detail: grabs a raw conn via db.Conn(ctx).Raw,
// type-asserts to *sqlite3.SQLiteConn, enables extension loading on that
// conn, and calls LoadExtension.
//
// Because Store pins MaxOpenConns=1 (SQLite WAL semantics), loading on
// the single pooled connection is sufficient. If in the future the pool
// grows, callers should re-register via a ConnectHook instead.
//
// Returns ErrExtensionMissing / ErrExtensionEmpty on placeholder builds
// so callers can fall back to brute-force cleanly.
func LoadInto(db *sql.DB, soPath string) error {
	if db == nil {
		return errors.New("sqlitevec: nil db")
	}
	if soPath == "" {
		return errors.New("sqlitevec: empty soPath")
	}

	info, err := os.Stat(soPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrExtensionMissing, soPath)
		}
		return fmt.Errorf("sqlitevec: stat %s: %w", soPath, err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%w: %s", ErrExtensionEmpty, soPath)
	}

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlitevec: acquire conn: %w", err)
	}
	defer conn.Close()

	return conn.Raw(func(driverConn any) error {
		sc, ok := driverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("sqlitevec: driver conn is %T, not *sqlite3.SQLiteConn (is the sqlite3 driver the one in use?)", driverConn)
		}
		// mattn's SQLiteConn.LoadExtension handles
		// sqlite3_enable_load_extension + sqlite3_load_extension
		// internally. The empty entrypoint string means sqlite3 auto-
		// discovers `sqlite3_<basename>_init`.
		if err := sc.LoadExtension(soPath, ""); err != nil {
			return fmt.Errorf("sqlitevec: LoadExtension(%s): %w", soPath, err)
		}
		return nil
	})
}
