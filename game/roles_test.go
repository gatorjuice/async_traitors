package game

import (
	"testing"

	"github.com/gatorjuice/async_traitors/db"
)

func TestAssignRoles_MinPlayers(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 4)

	if err := AssignRoles(database, nil, gameID); err != nil {
		t.Fatal(err)
	}

	traitors, _ := db.GetPlayersByRole(database, gameID, "traitor")
	faithful, _ := db.GetPlayersByRole(database, gameID, "faithful")

	if len(traitors) != 1 {
		t.Errorf("expected 1 traitor, got %d", len(traitors))
	}
	if len(faithful) != 3 {
		t.Errorf("expected 3 faithful, got %d", len(faithful))
	}
}

func TestAssignRoles_Proportions(t *testing.T) {
	database := testDB(t)

	// 8 players -> 2 traitors
	gameID := createTestGame(t, database, 8)
	if err := AssignRoles(database, nil, gameID); err != nil {
		t.Fatal(err)
	}

	traitors, _ := db.GetPlayersByRole(database, gameID, "traitor")
	if len(traitors) != 2 {
		t.Errorf("8 players: expected 2 traitors, got %d", len(traitors))
	}

	// 12 players -> 3 traitors
	gameID2 := createTestGame(t, database, 12)
	if err := AssignRoles(database, nil, gameID2); err != nil {
		t.Fatal(err)
	}

	traitors2, _ := db.GetPlayersByRole(database, gameID2, "traitor")
	if len(traitors2) != 3 {
		t.Errorf("12 players: expected 3 traitors, got %d", len(traitors2))
	}
}

func TestAssignRoles_TooFewPlayers(t *testing.T) {
	database := testDB(t)
	gameID := createTestGame(t, database, 3)

	err := AssignRoles(database, nil, gameID)
	if err == nil {
		t.Error("expected error for too few players")
	}
}

func TestAssignRoles_Randomness(t *testing.T) {
	database := testDB(t)

	// Run 20 times and check that traitor assignment isn't always the same
	firstTraitor := ""
	varied := false

	for i := 0; i < 20; i++ {
		gameID := createTestGame(t, database, 4)
		if err := AssignRoles(database, nil, gameID); err != nil {
			t.Fatal(err)
		}

		traitors, _ := db.GetPlayersByRole(database, gameID, "traitor")
		if len(traitors) != 1 {
			t.Fatalf("expected 1 traitor, got %d", len(traitors))
		}

		if firstTraitor == "" {
			firstTraitor = traitors[0].DiscordID
		} else if traitors[0].DiscordID != firstTraitor {
			varied = true
		}
	}

	if !varied {
		t.Error("traitor assignment was always the same player across 20 runs — randomness may be broken")
	}
}
