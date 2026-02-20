package game

import (
	"testing"

	"github.com/gatorjuice/async_traitors/db"
)

func TestResolveNight_Murder(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 4)
	pids := getPlayerIDs(t, database, gameID)
	db.UpdateGameStatus(database, gameID, "active", "night")
	db.UpdateGameRound(database, gameID, 1)
	db.UpdatePlayerRole(database, gameID, pids[0], "traitor")
	db.UpdatePlayerRole(database, gameID, pids[1], "faithful")
	db.UpdatePlayerRole(database, gameID, pids[2], "faithful")
	db.UpdatePlayerRole(database, gameID, pids[3], "faithful")

	db.CastVote(database, gameID, 1, "night", pids[0], pids[1])

	if err := ResolveNight(database, nil, gameID, 1); err != nil {
		t.Fatal(err)
	}

	p, _ := db.GetPlayer(database, gameID, pids[1])
	if p.Status != "murdered" {
		t.Errorf("expected murdered, got %s", p.Status)
	}
}

func TestResolveNight_ShieldBlock(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 4)
	pids := getPlayerIDs(t, database, gameID)
	db.UpdateGameStatus(database, gameID, "active", "night")
	db.UpdateGameRound(database, gameID, 1)
	db.UpdatePlayerRole(database, gameID, pids[0], "traitor")
	db.UpdatePlayerRole(database, gameID, pids[1], "faithful")

	db.GrantShield(database, gameID, pids[1], "test", 1)

	db.CastVote(database, gameID, 1, "night", pids[0], pids[1])

	if err := ResolveNight(database, nil, gameID, 1); err != nil {
		t.Fatal(err)
	}

	p, _ := db.GetPlayer(database, gameID, pids[1])
	if p.Status != "alive" {
		t.Errorf("expected alive (shielded), got %s", p.Status)
	}
	if p.HasShield {
		t.Error("expected shield to be consumed")
	}
}

func TestResolveNight_NoVotes(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 4)
	db.UpdateGameStatus(database, gameID, "active", "night")
	db.UpdateGameRound(database, gameID, 1)

	if err := ResolveNight(database, nil, gameID, 1); err != nil {
		t.Fatal(err)
	}

	alive, _ := db.GetAlivePlayers(database, gameID)
	if len(alive) != 4 {
		t.Errorf("expected 4 alive, got %d", len(alive))
	}
}
