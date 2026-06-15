package main

import (
	"database/sql"
	"fmt"
	"strings"
)

// hasColumn reports whether table contains a column named col.
// Uses SQLite's PRAGMA table_info to inspect the live schema.
func hasColumn(db *sql.DB, table, col string) (bool, error) {
	// PRAGMA table_info returns one row per column: (cid, name, type, notnull, dflt_value, pk).
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table)) // #nosec G201 -- table is an internal literal, never from user input
	if err != nil {
		return false, fmt.Errorf("PRAGMA table_info(%s): %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}

// addServerIDColumn adds a TEXT NOT NULL DEFAULT 'default' server_id column to
// table if it does not already exist. Uses the isDuplicateColumnErr guard as a
// belt-and-suspenders fallback (hasColumn is the primary guard).
func addServerIDColumn(db *sql.DB, table string) error {
	q := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN server_id TEXT NOT NULL DEFAULT 'default'`, table) // #nosec G201 -- table is an internal literal
	if _, err := db.Exec(q); err != nil && !isDuplicateColumnErr(err) {
		return fmt.Errorf("add server_id to %s: %w", table, err)
	}
	return nil
}

// rebuildTableWithServerID adds server_id as the leading PK column of table by
// creating a replacement table (tmpName), copying all existing rows with
// server_id='default', dropping the original, and renaming. The operation runs
// inside a single transaction so a mid-migration crash leaves the database
// either fully old or fully new — never half-migrated.
//
// newDDL must CREATE TABLE tmpName with server_id as the first PK column and
// the same non-PK data columns as table. cols is the ordered list of those
// data columns (server_id excluded) used to build the SELECT projection.
//
// The function is idempotent: if table already has a server_id column it
// returns immediately without touching anything.
func rebuildTableWithServerID(db *sql.DB, table, tmpName, newDDL string, cols []string) error {
	has, err := hasColumn(db, table, "server_id")
	if err != nil {
		return err
	}
	if has {
		return nil // already migrated
	}

	colList := strings.Join(cols, ", ")
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("rebuild %s: begin: %w", table, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(newDDL); err != nil {
		return fmt.Errorf("rebuild %s: create %s: %w", table, tmpName, err)
	}
	if _, err := tx.Exec(fmt.Sprintf(
		`INSERT INTO %s (server_id, %s) SELECT 'default', %s FROM %s`,
		tmpName, colList, colList, table,
	)); err != nil {
		return fmt.Errorf("rebuild %s: copy rows: %w", table, err)
	}
	if _, err := tx.Exec(`DROP TABLE ` + table); err != nil { // #nosec G201 -- table is an internal literal
		return fmt.Errorf("rebuild %s: drop original: %w", table, err)
	}
	if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, tmpName, table)); err != nil {
		return fmt.Errorf("rebuild %s: rename: %w", table, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("rebuild %s: commit: %w", table, err)
	}
	return nil
}
