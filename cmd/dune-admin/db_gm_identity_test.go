package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// captureExecer records every Exec call so tests can assert on the SQL and args
// without touching a real database. It satisfies the pgExecutor interface.
type captureExecer struct {
	calls []execCapture
}

type execCapture struct {
	sql  string
	args []any
}

func (c *captureExecer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	c.calls = append(c.calls, execCapture{sql: sql, args: args})
	return pgconn.CommandTag{}, nil
}

// TestSeedGMIdentity verifies the SQL execution sequence emitted by seedGMIdentity:
//  1. The player_state write uses a guarded INSERT ... SELECT ... WHERE NOT EXISTS
//     so the bare ON CONFLICT DO NOTHING (which stopped working after the game's
//     encrypted_player_state schema migration) can never create a duplicate.
//  2. A dedupe DELETE is issued before the insert to self-heal servers that already
//     accumulated duplicate GM rows (the director crash symptom reported in #267).
//  3. Both the DELETE and the guarded INSERT are scoped to the GM account id (9000001)
//     so no real player data is touched.
func TestSeedGMIdentity(t *testing.T) {
	t.Parallel()
	s := gmSeedSpec()
	db := &captureExecer{}
	if err := seedGMIdentity(context.Background(), db, s); err != nil {
		t.Fatalf("seedGMIdentity returned unexpected error: %v", err)
	}

	// Must have at least: 1 account + 3 actors + 1 delete + 1 player_state insert = 6 calls.
	if len(db.calls) < 6 {
		t.Fatalf("expected at least 6 Exec calls, got %d: %v", len(db.calls), db.calls)
	}

	t.Run("issues dedupe DELETE scoped to GM account_id", func(t *testing.T) {
		t.Parallel()
		var found bool
		for _, c := range db.calls {
			if strings.Contains(c.sql, "DELETE FROM dune.encrypted_player_state") &&
				strings.Contains(c.sql, "account_id") &&
				strings.Contains(c.sql, "MIN(id)") {
				// Verify the GM account id is the bound arg.
				found = true
				hasGMAccountID := false
				for _, arg := range c.args {
					if arg == s.AccountID {
						hasGMAccountID = true
					}
				}
				if !hasGMAccountID {
					t.Fatalf("dedupe DELETE does not bind GM account id %d; args: %v", s.AccountID, c.args)
				}
				break
			}
		}
		if !found {
			t.Fatal("no dedupe DELETE FROM dune.encrypted_player_state found in Exec calls")
		}
	})

	t.Run("player_state write uses WHERE NOT EXISTS not bare ON CONFLICT", func(t *testing.T) {
		t.Parallel()
		var found bool
		for _, c := range db.calls {
			if strings.Contains(c.sql, "INSERT INTO dune.encrypted_player_state") {
				found = true
				if strings.Contains(c.sql, "ON CONFLICT") {
					t.Fatal("player_state INSERT still uses bare ON CONFLICT DO NOTHING — must use WHERE NOT EXISTS guard")
				}
				if !strings.Contains(c.sql, "WHERE NOT EXISTS") {
					t.Fatal("player_state INSERT missing WHERE NOT EXISTS guard")
				}
			}
		}
		if !found {
			t.Fatal("no INSERT INTO dune.encrypted_player_state found in Exec calls")
		}
	})

	t.Run("actor inserts carry GM account_id as owner_account_id", func(t *testing.T) {
		t.Parallel()
		actorCalls := 0
		for _, c := range db.calls {
			if strings.Contains(c.sql, "INSERT INTO dune.actors") {
				actorCalls++
				hasGMAccountID := false
				for _, arg := range c.args {
					if arg == s.AccountID {
						hasGMAccountID = true
					}
				}
				if !hasGMAccountID {
					t.Fatalf("actor INSERT does not bind GM account id %d; args: %v", s.AccountID, c.args)
				}
			}
		}
		if actorCalls != 3 {
			t.Fatalf("expected 3 actor INSERT calls, got %d", actorCalls)
		}
	})
}

// TestGMSeedSpec locks the recon-derived seed values for the GM/Server persona:
// the sentinel ids (collision-free per Phase 0 recon), the exact actor class paths
// the live schema uses (or the game's player-info lookup fails and the sender never
// renders), and the blast-radius-safe defaults (Offline status; the seed routine
// leaves actors.transform NULL so the GM never plots on the live map).
func TestGMSeedSpec(t *testing.T) {
	t.Parallel()
	s := gmSeedSpec()

	if s.AccountID != gmIdentityAccountID {
		t.Fatalf("AccountID = %d, want %d", s.AccountID, gmIdentityAccountID)
	}
	// Actor ids derive from the account id: 9000001 -> 900000101/02/03.
	if s.ControllerID != 900000101 || s.StateID != 900000102 || s.PawnID != 900000103 {
		t.Fatalf("actor ids wrong: %d/%d/%d", s.ControllerID, s.StateID, s.PawnID)
	}
	if !strings.Contains(s.ControllerClass, "BP_DunePlayerController") {
		t.Fatalf("controller class wrong: %s", s.ControllerClass)
	}
	if !strings.Contains(s.StateClass, "DunePlayerState") {
		t.Fatalf("state class wrong: %s", s.StateClass)
	}
	if !strings.Contains(s.PawnClass, "BP_DunePlayerCharacter") {
		t.Fatalf("pawn class wrong: %s", s.PawnClass)
	}
	// Blast-radius: Offline keeps the GM out of the online pollers / welcome scanner.
	if s.OnlineStatus != "Offline" {
		t.Fatalf("OnlineStatus = %q, want Offline", s.OnlineStatus)
	}
	if s.LifeState != "Alive" {
		t.Fatalf("LifeState = %q, want Alive", s.LifeState)
	}
	if s.FuncomID != "GM#0001" || s.CharacterName != "GM" {
		t.Fatalf("persona wrong: funcom=%q char=%q", s.FuncomID, s.CharacterName)
	}
	if s.Map != "HaggaBasin" || s.PartitionID != 1 {
		t.Fatalf("location wrong: map=%q partition=%d", s.Map, s.PartitionID)
	}
}
