package main

import (
	"database/sql"
	"testing"
	"time"
)

// openSharedScopeDB opens a shared in-memory SQLite DB and applies the unified
// schema. Used by server-scope isolation tests.
func openSharedScopeDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open shared db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := applyUnifiedSchema(db); err != nil {
		t.Fatalf("applyUnifiedSchema: %v", err)
	}
	return db
}

// ── welcome_grants ─────────────────────────────────────────────────────────────

// TestWelcomeStore_GrantsServerScope verifies that two welcomeStore instances
// sharing the same DB cannot see each other's welcome_grants rows.
func TestWelcomeStore_GrantsServerScope(t *testing.T) {
	t.Parallel()
	db := openSharedScopeDB(t)

	sA := newWelcomeStore(db, "srv-a")
	sB := newWelcomeStore(db, "srv-b")

	if err := sA.insertGranted("FLS1", "v1", 1, "Paul"); err != nil {
		t.Fatalf("sA.insertGranted: %v", err)
	}

	// sB must not see sA's grant.
	if ex, err := sB.grantExists("FLS1", "v1", 1); err != nil {
		t.Fatalf("sB.grantExists: %v", err)
	} else if ex {
		t.Error("server B should not see server A's grant")
	}

	// sA must see its own grant.
	if ex, err := sA.grantExists("FLS1", "v1", 1); err != nil {
		t.Fatalf("sA.grantExists: %v", err)
	} else if !ex {
		t.Error("server A should see its own grant")
	}

	// listGrants on sB returns nothing.
	recs, err := sB.listGrants(10)
	if err != nil {
		t.Fatalf("sB.listGrants: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("sB.listGrants: got %d rows, want 0", len(recs))
	}
}

// ── welcome_config ─────────────────────────────────────────────────────────────

// TestWelcomeStore_ConfigServerScope verifies that saveConfig on server A is
// invisible to server B's loadConfig.
func TestWelcomeStore_ConfigServerScope(t *testing.T) {
	t.Parallel()
	db := openSharedScopeDB(t)

	sA := newWelcomeStore(db, "srv-a")
	sB := newWelcomeStore(db, "srv-b")

	cfg := welcomeConfigRow{Enabled: true, ScanSecs: 99}
	if err := sA.saveConfig(cfg); err != nil {
		t.Fatalf("sA.saveConfig: %v", err)
	}

	_, ok, err := sB.loadConfig()
	if err != nil {
		t.Fatalf("sB.loadConfig: %v", err)
	}
	if ok {
		t.Error("server B should not see server A's config (ok must be false)")
	}

	row, ok, err := sA.loadConfig()
	if err != nil {
		t.Fatalf("sA.loadConfig: %v", err)
	}
	if !ok {
		t.Error("server A should see its own config")
	}
	if row.ScanSecs != 99 {
		t.Errorf("sA.loadConfig ScanSecs = %d, want 99", row.ScanSecs)
	}
}

// ── give_packs_config ──────────────────────────────────────────────────────────

// TestGivePacksStore_ServerScope verifies that saveConfig on server A is
// invisible to server B's loadConfig.
func TestGivePacksStore_ServerScope(t *testing.T) {
	t.Parallel()
	db := openSharedScopeDB(t)

	sA := newGivePacksStore(db, "srv-a")
	sB := newGivePacksStore(db, "srv-b")

	const packsJSON = `[{"id":"p1"}]`
	if err := sA.saveConfig(packsJSON, true); err != nil {
		t.Fatalf("sA.saveConfig: %v", err)
	}

	_, _, ok, err := sB.loadConfig()
	if err != nil {
		t.Fatalf("sB.loadConfig: %v", err)
	}
	if ok {
		t.Error("server B should not see server A's config (ok must be false)")
	}

	_, got, ok, err := sA.loadConfig()
	if err != nil {
		t.Fatalf("sA.loadConfig: %v", err)
	}
	if !ok {
		t.Error("server A should see its own config")
	}
	if got != packsJSON {
		t.Errorf("sA.loadConfig packs = %q, want %q", got, packsJSON)
	}
}

// ── event_award_claims ─────────────────────────────────────────────────────────

// TestEventStore_ClaimsServerScope verifies that a claim recorded on server A
// is not visible from server B.
func TestEventStore_ClaimsServerScope(t *testing.T) {
	t.Parallel()
	db := openSharedScopeDB(t)

	sA := newEventStore(db, "srv-a")
	sB := newEventStore(db, "srv-b")

	if err := sA.recordGranted(1, 1, 42); err != nil {
		t.Fatalf("sA.recordGranted: %v", err)
	}

	// sB must not see the claim.
	ex, err := sB.claimExists(1, 1, 42)
	if err != nil {
		t.Fatalf("sB.claimExists: %v", err)
	}
	if ex {
		t.Error("server B should not see server A's claim")
	}

	// sA must see its own claim.
	ex, err = sA.claimExists(1, 1, 42)
	if err != nil {
		t.Fatalf("sA.claimExists: %v", err)
	}
	if !ex {
		t.Error("server A should see its own claim")
	}
}

// ── battlepass_claims ──────────────────────────────────────────────────────────

// TestBattlepassStore_ClaimsServerScope verifies that a claim recorded on
// server A is invisible to server B.
func TestBattlepassStore_ClaimsServerScope(t *testing.T) {
	t.Parallel()
	db := openSharedScopeDB(t)

	sA := newBattlepassStore(db, "srv-a")
	sB := newBattlepassStore(db, "srv-b")

	if err := sA.recordClaim("level:5", 42, 100, battlepassClaimEarned); err != nil {
		t.Fatalf("sA.recordClaim: %v", err)
	}

	// sB must not see the claim.
	keys, err := sB.claimedKeys(42)
	if err != nil {
		t.Fatalf("sB.claimedKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("server B should not see server A's claims, got %v", keys)
	}

	// sA must see its own claim.
	keys, err = sA.claimedKeys(42)
	if err != nil {
		t.Fatalf("sA.claimedKeys: %v", err)
	}
	if _, ok := keys["level:5"]; !ok {
		t.Error("server A should see its own claim")
	}
}

// ── battlepass_grant_ledger ────────────────────────────────────────────────────

// TestBattlepassStore_GrantLedgerServerScope verifies that a pending grant
// recorded on server A is not visible from server B's retry list.
func TestBattlepassStore_GrantLedgerServerScope(t *testing.T) {
	t.Parallel()
	db := openSharedScopeDB(t)

	sA := newBattlepassStore(db, "srv-a")
	sB := newBattlepassStore(db, "srv-b")

	if err := sA.recordPendingGrant("level:5", 42); err != nil {
		t.Fatalf("sA.recordPendingGrant: %v", err)
	}

	// sB's retry list must be empty.
	rows, err := sB.listRetryableGrantLedger(time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("sB.listRetryableGrantLedger: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("server B should not see server A's grant ledger, got %d rows", len(rows))
	}

	// sA must see its own pending grant.
	rows, err = sA.listRetryableGrantLedger(time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("sA.listRetryableGrantLedger: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("server A should see 1 pending grant, got %d", len(rows))
	}
}

// ── discord status meta key ───────────────────────────────────────────────────

// TestStatusMessageMetaKeyNamespace verifies that statusMessageMetaKey returns
// distinct keys per server and is namespaced with the server ID.
func TestStatusMessageMetaKeyNamespace(t *testing.T) {
	t.Parallel()
	keyA := statusMessageMetaKey("srv-a")
	keyB := statusMessageMetaKey("srv-b")
	if keyA == keyB {
		t.Error("meta keys for different servers must be distinct")
	}
	want := "discord_status_message:srv-a"
	if keyA != want {
		t.Errorf("statusMessageMetaKey(%q) = %q, want %q", "srv-a", keyA, want)
	}
}

// TestSqliteStatusStore_ServerScope verifies that status message saved by
// server A is not visible to server B.
func TestSqliteStatusStore_ServerScope(t *testing.T) {
	t.Parallel()
	db := openSharedScopeDB(t)

	ssA := newSqliteStatusStore(db, "srv-a")
	ssB := newSqliteStatusStore(db, "srv-b")

	if err := ssA.saveStatusMessage("ch1", "msg1"); err != nil {
		t.Fatalf("ssA.saveStatusMessage: %v", err)
	}

	ch, msg, err := ssB.loadStatusMessage()
	if err != nil {
		t.Fatalf("ssB.loadStatusMessage: %v", err)
	}
	if ch != "" || msg != "" {
		t.Errorf("server B should not see server A's status message, got ch=%q msg=%q", ch, msg)
	}

	ch, msg, err = ssA.loadStatusMessage()
	if err != nil {
		t.Fatalf("ssA.loadStatusMessage: %v", err)
	}
	if ch != "ch1" || msg != "msg1" {
		t.Errorf("server A status message = (%q, %q), want (ch1, msg1)", ch, msg)
	}
}
