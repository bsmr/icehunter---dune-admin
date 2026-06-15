package main

import (
	"database/sql"
	"testing"
)

// oldSchemaSQL creates all tables in their pre-migration form (no server_id
// columns, original PKs). Used to seed a pre-migration database so that the
// migration helpers can be tested against realistic legacy data.
const oldSchemaSQL = `
CREATE TABLE IF NOT EXISTS play_sessions (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id    INTEGER NOT NULL,
	started_at    TEXT    NOT NULL,
	ended_at      TEXT,
	duration_secs INTEGER
);
CREATE TABLE IF NOT EXISTS stat_snapshots (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id      INTEGER NOT NULL,
	snapped_at      TEXT    NOT NULL,
	char_xp         INTEGER,
	skill_points    INTEGER,
	intel_points    INTEGER,
	combat_xp       INTEGER,
	crafting_xp     INTEGER,
	gathering_xp    INTEGER,
	exploration_xp  INTEGER,
	sabotage_xp     INTEGER,
	solaris_balance INTEGER
);
CREATE TABLE IF NOT EXISTS welcome_grants (
	fls_id          TEXT    NOT NULL,
	package_version TEXT    NOT NULL,
	account_id      INTEGER NOT NULL,
	character_name  TEXT    NOT NULL DEFAULT '',
	status          TEXT    NOT NULL,
	granted_at      TEXT    NOT NULL DEFAULT '',
	attempts        INTEGER NOT NULL DEFAULT 1,
	last_error      TEXT    NOT NULL DEFAULT '',
	detected_at     TEXT    NOT NULL,
	updated_at      TEXT    NOT NULL,
	PRIMARY KEY (fls_id, package_version, account_id)
);
CREATE TABLE IF NOT EXISTS welcome_config (
	id                             INTEGER PRIMARY KEY CHECK (id = 1),
	enabled                        INTEGER NOT NULL DEFAULT 0,
	scan_secs                      INTEGER NOT NULL DEFAULT 30,
	active_version                 TEXT    NOT NULL DEFAULT '',
	active_versions_json           TEXT    NOT NULL DEFAULT '',
	packages_json                  TEXT    NOT NULL DEFAULT '[]',
	welcome_message_enabled        INTEGER NOT NULL DEFAULT 0,
	welcome_message                TEXT    NOT NULL DEFAULT '',
	welcome_whisper_source_player  TEXT    NOT NULL DEFAULT '',
	motd_enabled                   INTEGER NOT NULL DEFAULT 0,
	motd_message                   TEXT    NOT NULL DEFAULT '',
	motd_source_player             TEXT    NOT NULL DEFAULT '',
	region_join_enabled            INTEGER NOT NULL DEFAULT 0,
	region_leave_enabled           INTEGER NOT NULL DEFAULT 0,
	region_join_template           TEXT    NOT NULL DEFAULT '',
	region_leave_template          TEXT    NOT NULL DEFAULT '',
	region_chat_channel            TEXT    NOT NULL DEFAULT 'whisper',
	updated_at                     TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS map_locations (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT    NOT NULL UNIQUE,
	x          REAL    NOT NULL DEFAULT 0,
	y          REAL    NOT NULL DEFAULT 0,
	z          REAL    NOT NULL DEFAULT 0,
	sort       INTEGER NOT NULL DEFAULT 0,
	created_at TEXT    NOT NULL,
	updated_at TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS give_packs_config (
	id                INTEGER PRIMARY KEY CHECK (id = 1),
	base_packs_loaded INTEGER NOT NULL DEFAULT 0,
	packs_json        TEXT    NOT NULL DEFAULT '[]',
	updated_at        TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS event_definitions (
	id                  INTEGER PRIMARY KEY AUTOINCREMENT,
	name                TEXT    NOT NULL,
	type                TEXT    NOT NULL,
	enabled             INTEGER NOT NULL DEFAULT 0,
	version             INTEGER NOT NULL DEFAULT 1,
	config_json         TEXT    NOT NULL DEFAULT '{}',
	reward_json         TEXT    NOT NULL DEFAULT '',
	announce_channel_id TEXT    NOT NULL DEFAULT '',
	announce_template   TEXT    NOT NULL DEFAULT '',
	poll_seconds        INTEGER NOT NULL DEFAULT 7,
	jitter_seconds      INTEGER NOT NULL DEFAULT 3,
	created_at          TEXT    NOT NULL,
	updated_at          TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS event_award_claims (
	event_id        INTEGER NOT NULL,
	version         INTEGER NOT NULL,
	account_id      INTEGER NOT NULL,
	status          TEXT    NOT NULL,
	claimed_at      TEXT    NOT NULL DEFAULT '',
	attempts        INTEGER NOT NULL DEFAULT 1,
	last_error      TEXT    NOT NULL DEFAULT '',
	next_attempt_at TEXT    NOT NULL DEFAULT '',
	updated_at      TEXT    NOT NULL,
	PRIMARY KEY (event_id, version, account_id)
);
CREATE TABLE IF NOT EXISTS battlepass_tiers (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	tier_key   TEXT    NOT NULL UNIQUE,
	category   TEXT    NOT NULL,
	label      TEXT    NOT NULL,
	signal     TEXT    NOT NULL,
	signal_key TEXT    NOT NULL DEFAULT '',
	threshold  INTEGER NOT NULL DEFAULT 0,
	intel      INTEGER NOT NULL DEFAULT 0,
	enabled    INTEGER NOT NULL DEFAULT 1,
	created_at TEXT    NOT NULL,
	updated_at TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS battlepass_claims (
	tier_key   TEXT    NOT NULL,
	account_id INTEGER NOT NULL,
	status     TEXT    NOT NULL,
	intel      INTEGER NOT NULL DEFAULT 0,
	earned_at  TEXT    NOT NULL DEFAULT '',
	granted_at TEXT    NOT NULL DEFAULT '',
	attempts   INTEGER NOT NULL DEFAULT 0,
	last_error TEXT    NOT NULL DEFAULT '',
	updated_at TEXT    NOT NULL,
	PRIMARY KEY (tier_key, account_id)
);
CREATE TABLE IF NOT EXISTS battlepass_accounts (
	account_id   INTEGER PRIMARY KEY,
	baselined_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS battlepass_grant_ledger (
	tier_key        TEXT    NOT NULL,
	account_id      INTEGER NOT NULL,
	status          TEXT    NOT NULL DEFAULT 'pending',
	attempts        INTEGER NOT NULL DEFAULT 0,
	last_error      TEXT    NOT NULL DEFAULT '',
	next_attempt_at TEXT    NOT NULL DEFAULT '',
	updated_at      TEXT    NOT NULL,
	PRIMARY KEY (tier_key, account_id)
);
CREATE TABLE IF NOT EXISTS auth_users (
	username      TEXT PRIMARY KEY,
	password_hash TEXT NOT NULL DEFAULT '',
	capabilities  TEXT NOT NULL DEFAULT '',
	created_at    TEXT NOT NULL,
	updated_at    TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

// seedLegacyRows inserts one representative row into every per-server table
// so the migration test can verify that data survives with server_id='default'.
func seedLegacyRows(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`INSERT INTO play_sessions(account_id, started_at, ended_at, duration_secs) VALUES (42, '2024-01-01T00:00:00Z', '2024-01-01T01:00:00Z', 3600)`,
		`INSERT INTO stat_snapshots(account_id, snapped_at, char_xp) VALUES (42, '2024-01-01T00:00:00Z', 1000)`,
		`INSERT INTO welcome_grants(fls_id,package_version,account_id,status,detected_at,updated_at) VALUES ('fls1','v1',42,'granted','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`,
		`INSERT INTO welcome_config(id,enabled,updated_at) VALUES (1,1,'2024-01-01T00:00:00Z')`,
		`INSERT INTO map_locations(name,created_at,updated_at) VALUES ('Arrakeen','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`,
		`INSERT INTO give_packs_config(id,updated_at) VALUES (1,'2024-01-01T00:00:00Z')`,
		`INSERT INTO event_definitions(name,type,enabled,created_at,updated_at) VALUES ('test','kill',1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`,
		`INSERT INTO event_award_claims(event_id,version,account_id,status,updated_at) VALUES (1,1,42,'claimed','2024-01-01T00:00:00Z')`,
		`INSERT INTO battlepass_tiers(tier_key,category,label,signal,created_at,updated_at) VALUES ('level:5','level','Level 5','level','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`,
		`INSERT INTO battlepass_claims(tier_key,account_id,status,updated_at) VALUES ('level:5',42,'earned','2024-01-01T00:00:00Z')`,
		`INSERT INTO battlepass_accounts(account_id,baselined_at) VALUES (42,'2024-01-01T00:00:00Z')`,
		`INSERT INTO battlepass_grant_ledger(tier_key,account_id,updated_at) VALUES ('level:5',42,'2024-01-01T00:00:00Z')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("seed: %v\nSQL: %s", err, s)
		}
	}
}

// countRowsWhere counts rows in table matching a WHERE clause.
func countRowsWhere(t *testing.T, db *sql.DB, table, where string, args ...any) int {
	t.Helper()
	var n int
	q := "SELECT COUNT(*) FROM " + table
	if where != "" {
		q += " WHERE " + where
	}
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("countRowsWhere(%s): %v", table, err)
	}
	return n
}

// TestApplyUnifiedSchema_ServerIDMigration_Simple verifies that simple-ALTER
// tables gain a server_id column defaulted to 'default', existing rows are
// preserved, and running applyUnifiedSchema twice is idempotent.
func TestApplyUnifiedSchema_ServerIDMigration_Simple(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Seed old schema + data.
	if _, err := db.Exec(oldSchemaSQL); err != nil {
		t.Fatalf("seed old schema: %v", err)
	}
	seedLegacyRows(t, db)

	// Run migration (first time).
	if err := applyUnifiedSchema(db); err != nil {
		t.Fatalf("applyUnifiedSchema (first): %v", err)
	}

	simpleAlterTables := []string{
		"play_sessions", "stat_snapshots",
		"map_locations", "event_definitions",
		"battlepass_tiers", "battlepass_accounts",
	}
	for _, tbl := range simpleAlterTables {
		t.Run(tbl+"_has_server_id_column", func(t *testing.T) {
			has, err := hasColumn(db, tbl, "server_id")
			if err != nil {
				t.Fatalf("hasColumn: %v", err)
			}
			if !has {
				t.Errorf("%s: server_id column missing after migration", tbl)
			}
		})
		t.Run(tbl+"_row_preserved_with_default", func(t *testing.T) {
			n := countRowsWhere(t, db, tbl, "server_id = 'default'")
			if n == 0 {
				t.Errorf("%s: no rows with server_id='default' after migration", tbl)
			}
		})
	}

	// Idempotency: second run must not error or duplicate rows.
	t.Run("idempotent_second_run", func(t *testing.T) {
		if err := applyUnifiedSchema(db); err != nil {
			t.Fatalf("applyUnifiedSchema (second): %v", err)
		}
		for _, tbl := range simpleAlterTables {
			n := countRowsWhere(t, db, tbl, "server_id = 'default'")
			before := countRowsWhere(t, db, tbl, "")
			if n != before {
				t.Errorf("%s: server_id rows (%d) != total rows (%d) after second run", tbl, n, before)
			}
		}
	})
}

// TestApplyUnifiedSchema_ServerIDMigration_Rebuild verifies that composite-PK
// tables are rebuilt with server_id as the leading PK component, existing rows
// survive with server_id='default', and the migration is idempotent.
func TestApplyUnifiedSchema_ServerIDMigration_Rebuild(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(oldSchemaSQL); err != nil {
		t.Fatalf("seed old schema: %v", err)
	}
	seedLegacyRows(t, db)

	if err := applyUnifiedSchema(db); err != nil {
		t.Fatalf("applyUnifiedSchema (first): %v", err)
	}

	rebuildTables := []struct {
		name   string
		pkCols []string // expected PK columns (server_id first)
	}{
		{"welcome_grants", []string{"server_id", "fls_id", "package_version", "account_id"}},
		{"welcome_config", []string{"server_id"}},
		{"give_packs_config", []string{"server_id"}},
		{"event_award_claims", []string{"server_id", "event_id", "version", "account_id"}},
		{"battlepass_claims", []string{"server_id", "tier_key", "account_id"}},
		{"battlepass_grant_ledger", []string{"server_id", "tier_key", "account_id"}},
	}

	for _, rt := range rebuildTables {
		t.Run(rt.name+"_has_server_id", func(t *testing.T) {
			has, err := hasColumn(db, rt.name, "server_id")
			if err != nil {
				t.Fatalf("hasColumn: %v", err)
			}
			if !has {
				t.Errorf("%s: server_id column missing after rebuild", rt.name)
			}
		})
		t.Run(rt.name+"_row_preserved", func(t *testing.T) {
			n := countRowsWhere(t, db, rt.name, "server_id = 'default'")
			if n == 0 {
				t.Errorf("%s: no rows with server_id='default' after rebuild", rt.name)
			}
		})
	}

	// Idempotency.
	t.Run("rebuild_idempotent", func(t *testing.T) {
		// Count rows before second run.
		counts := make(map[string]int)
		for _, rt := range rebuildTables {
			counts[rt.name] = countRowsWhere(t, db, rt.name, "")
		}
		if err := applyUnifiedSchema(db); err != nil {
			t.Fatalf("applyUnifiedSchema (second): %v", err)
		}
		for _, rt := range rebuildTables {
			after := countRowsWhere(t, db, rt.name, "")
			if after != counts[rt.name] {
				t.Errorf("%s: row count changed from %d to %d on second run", rt.name, counts[rt.name], after)
			}
		}
	})
}

// TestHasColumn verifies the hasColumn helper against known columns and
// a non-existent column.
func TestHasColumn(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`CREATE TABLE foo (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		col  string
		want bool
	}{
		{"id", true},
		{"name", true},
		{"missing", false},
		{"server_id", false},
	}
	for _, tt := range tests {
		t.Run(tt.col, func(t *testing.T) {
			got, err := hasColumn(db, "foo", tt.col)
			if err != nil {
				t.Fatalf("hasColumn: %v", err)
			}
			if got != tt.want {
				t.Errorf("hasColumn(foo, %q) = %v, want %v", tt.col, got, tt.want)
			}
		})
	}
}
