package game

import (
	"testing"

	"github.com/gatorjuice/async_traitors/db"
)

func TestRevealAboveThreshold(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 6)
	pids := getPlayerIDs(t, database, gameID)
	db.UpdatePlayerRole(database, gameID, pids[0], "traitor")

	// Default threshold is 4, we have 6 alive — should reveal
	err := RevealRole(database, nil, gameID, pids[0])
	if err != nil {
		t.Fatal(err)
	}
}

func TestRevealAtThreshold(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 4)
	pids := getPlayerIDs(t, database, gameID)
	db.UpdatePlayerRole(database, gameID, pids[0], "traitor")

	// 4 alive == threshold of 4, should NOT reveal
	err := RevealRole(database, nil, gameID, pids[0])
	if err != nil {
		t.Fatal(err)
	}
}
