package game

import (
	"testing"

	"github.com/gatorjuice/async_traitors/db"
)

func TestTallyVotes_ClearWinner(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 4)
	db.UpdateGameStatus(database, gameID, "active", "roundtable")
	db.UpdateGameRound(database, gameID, 1)

	pids := getPlayerIDs(t, database, gameID)

	// 3 votes for pids[0]
	db.CastVote(database, gameID, 1, "roundtable", pids[1], pids[0])
	db.CastVote(database, gameID, 1, "roundtable", pids[2], pids[0])
	db.CastVote(database, gameID, 1, "roundtable", pids[3], pids[0])

	banished, err := TallyBanishmentVotes(database, nil, gameID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if banished != pids[0] {
		t.Errorf("expected %s banished, got %s", pids[0], banished)
	}

	p, _ := db.GetPlayer(database, gameID, pids[0])
	if p.Status != "banished" {
		t.Errorf("expected banished status, got %s", p.Status)
	}
}

func TestTallyVotes_Tie(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 4)
	db.UpdateGameStatus(database, gameID, "active", "roundtable")
	db.UpdateGameRound(database, gameID, 1)

	pids := getPlayerIDs(t, database, gameID)

	// 1 vote each — tie
	db.CastVote(database, gameID, 1, "roundtable", pids[2], pids[0])
	db.CastVote(database, gameID, 1, "roundtable", pids[3], pids[1])

	banished, err := TallyBanishmentVotes(database, nil, gameID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if banished != "" {
		t.Errorf("expected no banishment on tie, got %s", banished)
	}
}

func TestTallyVotes_NoVotes(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 4)
	db.UpdateGameStatus(database, gameID, "active", "roundtable")
	db.UpdateGameRound(database, gameID, 1)

	banished, err := TallyBanishmentVotes(database, nil, gameID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if banished != "" {
		t.Errorf("expected no banishment with no votes, got %s", banished)
	}
}
