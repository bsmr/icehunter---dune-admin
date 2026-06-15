package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // pure-Go sqlite driver (registers "sqlite")
)

// givePacksStore persists the operator-configurable give-items pack library in
// a local SQLite database. Kept in our own file so we never touch Funcom's
// dune schema. Mirrors welcomeStore / locationStore in structure and intent.
type givePacksStore struct {
	db       *sql.DB
	serverID string
}

const givePacksStoreSchema = `
CREATE TABLE IF NOT EXISTS give_packs_config (
	server_id         TEXT    NOT NULL DEFAULT 'default',
	base_packs_loaded INTEGER NOT NULL DEFAULT 0,
	packs_json        TEXT    NOT NULL DEFAULT '[]',
	updated_at        TEXT    NOT NULL DEFAULT '',
	PRIMARY KEY (server_id)
);`

// initGivePacksSchema creates the give_packs_config table on db. Safe to call
// against a shared handle (the unified store). Idempotent.
func initGivePacksSchema(db *sql.DB) error {
	if _, err := db.Exec(givePacksStoreSchema); err != nil {
		return fmt.Errorf("init give-packs schema: %w", err)
	}
	if err := rebuildTableWithServerID(db, "give_packs_config", "give_packs_config_new",
		`CREATE TABLE give_packs_config_new (
			server_id         TEXT    NOT NULL DEFAULT 'default',
			base_packs_loaded INTEGER NOT NULL DEFAULT 0,
			packs_json        TEXT    NOT NULL DEFAULT '[]',
			updated_at        TEXT    NOT NULL DEFAULT '',
			PRIMARY KEY (server_id)
		)`,
		[]string{"base_packs_loaded", "packs_json", "updated_at"},
	); err != nil {
		return fmt.Errorf("migrate give_packs_config server_id: %w", err)
	}
	return nil
}

// newGivePacksStore wraps an already-initialised shared handle (schema created
// by openUnifiedStore). Used so all stores share one SQLite file in production.
func newGivePacksStore(db *sql.DB, serverID string) *givePacksStore {
	return &givePacksStore{db: db, serverID: serverID}
}

// openGivePacksStore opens (or creates) the give-packs database at path and
// ensures the schema exists. path may be ":memory:" for tests.
func openGivePacksStore(path string) (*givePacksStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open give-packs store: %w", err)
	}
	if err := initGivePacksSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &givePacksStore{db: db, serverID: "default"}, nil
}

func (s *givePacksStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// saveConfig upserts the single give_packs_config row (id=1).
// packsJSON must be a valid JSON array (never nil — use "[]" for empty).
// basePacksLoaded=true means the default seed has been applied; subsequent
// startups will skip re-seeding even when packsJSON is "[]" (user deleted all).
func (s *givePacksStore) saveConfig(packsJSON string, basePacksLoaded bool) error {
	loaded := 0
	if basePacksLoaded {
		loaded = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO give_packs_config (server_id, base_packs_loaded, packs_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(server_id) DO UPDATE SET
			base_packs_loaded = excluded.base_packs_loaded,
			packs_json        = excluded.packs_json,
			updated_at        = excluded.updated_at`,
		s.serverID, loaded, packsJSON, now)
	if err != nil {
		return fmt.Errorf("save give-packs config: %w", err)
	}
	return nil
}

// loadConfig reads the single give_packs_config row.
// Returns (basePacksLoaded, packsJSON, ok, err).
// ok=false when the table is empty (first boot); in that case the caller
// should seed from the embedded default.
func (s *givePacksStore) loadConfig() (basePacksLoaded bool, packsJSON string, ok bool, err error) {
	var loadedInt int
	scanErr := s.db.QueryRow(`
		SELECT base_packs_loaded, packs_json FROM give_packs_config WHERE server_id = ?`,
		s.serverID).Scan(&loadedInt, &packsJSON)
	if errors.Is(scanErr, sql.ErrNoRows) {
		return false, "", false, nil
	}
	if scanErr != nil {
		return false, "", false, fmt.Errorf("load give-packs config: %w", scanErr)
	}
	return loadedInt != 0, packsJSON, true, nil
}
