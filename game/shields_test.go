package game

import (
	"testing"

	"github.com/gatorjuice/async_traitors/db"
)

func TestGrantAndConsumeShield(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 4)
	pids := getPlayerIDs(t, database, gameID)

	db.GrantShield(database, gameID, pids[0], "test", 1)

	p, _ := db.GetPlayer(database, gameID, pids[0])
	if !p.HasShield {
		t.Error("expected has_shield=true after grant")
	}

	db.ConsumeShield(database, gameID, pids[0], 2)

	p, _ = db.GetPlayer(database, gameID, pids[0])
	if p.HasShield {
		t.Error("expected has_shield=false after consume")
	}
}

func TestConsumeShieldWithout(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 4)
	pids := getPlayerIDs(t, database, gameID)

	err := db.ConsumeShield(database, gameID, pids[0], 1)
	if err == nil {
		t.Error("expected error consuming shield when player has none")
	}
}

func TestShieldLogEntries(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 4)
	pids := getPlayerIDs(t, database, gameID)

	db.GrantShield(database, gameID, pids[0], "competition", 1)
	db.ConsumeShield(database, gameID, pids[0], 2)

	log, err := db.GetShieldLog(database, gameID)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(log))
	}
	if log[0].Source != "competition" {
		t.Errorf("expected source=competition, got %s", log[0].Source)
	}
	if log[0].RoundGranted != 1 {
		t.Errorf("expected round_granted=1, got %d", log[0].RoundGranted)
	}
	if log[0].RoundUsed == nil || *log[0].RoundUsed != 2 {
		t.Error("expected round_used=2")
	}
}
