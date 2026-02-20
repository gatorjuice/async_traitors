package db

import "testing"

func TestAddAndGetPlayer(t *testing.T) {
	db := testDB(t)
	id, _ := CreateGame(db, "PL1234", "g1", "c1", "u1")

	if err := AddPlayer(db, id, "player1", "Alice"); err != nil {
		t.Fatal(err)
	}

	p, err := GetPlayer(db, id, "player1")
	if err != nil {
		t.Fatal(err)
	}
	if p.DiscordName != "Alice" {
		t.Errorf("expected Alice, got %s", p.DiscordName)
	}
	if p.Role != "unassigned" {
		t.Errorf("expected unassigned, got %s", p.Role)
	}
	if p.Status != "alive" {
		t.Errorf("expected alive, got %s", p.Status)
	}
}

func TestGetAlivePlayers(t *testing.T) {
	db := testDB(t)
	id, _ := CreateGame(db, "AL1234", "g1", "c1", "u1")

	AddPlayer(db, id, "p1", "Alice")
	AddPlayer(db, id, "p2", "Bob")
	AddPlayer(db, id, "p3", "Carol")

	UpdatePlayerStatus(db, id, "p2", "murdered")

	alive, err := GetAlivePlayers(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(alive) != 2 {
		t.Errorf("expected 2 alive, got %d", len(alive))
	}
}

func TestGetPlayersByRole(t *testing.T) {
	db := testDB(t)
	id, _ := CreateGame(db, "RL1234", "g1", "c1", "u1")

	AddPlayer(db, id, "p1", "Alice")
	AddPlayer(db, id, "p2", "Bob")
	AddPlayer(db, id, "p3", "Carol")

	UpdatePlayerRole(db, id, "p1", "traitor")
	UpdatePlayerRole(db, id, "p2", "faithful")
	UpdatePlayerRole(db, id, "p3", "faithful")

	traitors, err := GetPlayersByRole(db, id, "traitor")
	if err != nil {
		t.Fatal(err)
	}
	if len(traitors) != 1 {
		t.Errorf("expected 1 traitor, got %d", len(traitors))
	}

	faithful, err := GetPlayersByRole(db, id, "faithful")
	if err != nil {
		t.Fatal(err)
	}
	if len(faithful) != 2 {
		t.Errorf("expected 2 faithful, got %d", len(faithful))
	}
}

func TestUpdatePlayerShield(t *testing.T) {
	db := testDB(t)
	id, _ := CreateGame(db, "SH1234", "g1", "c1", "u1")

	AddPlayer(db, id, "p1", "Alice")
	UpdatePlayerShield(db, id, "p1", true)

	p, _ := GetPlayer(db, id, "p1")
	if !p.HasShield {
		t.Error("expected has_shield=true")
	}

	UpdatePlayerShield(db, id, "p1", false)
	p, _ = GetPlayer(db, id, "p1")
	if p.HasShield {
		t.Error("expected has_shield=false")
	}
}

func TestDuplicatePlayerRejected(t *testing.T) {
	db := testDB(t)
	id, _ := CreateGame(db, "DU1234", "g1", "c1", "u1")

	AddPlayer(db, id, "p1", "Alice")
	err := AddPlayer(db, id, "p1", "Alice2")
	if err == nil {
		t.Error("expected error for duplicate player")
	}
}
