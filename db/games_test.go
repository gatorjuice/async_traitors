package db

import "testing"

func TestCreateAndGetGame(t *testing.T) {
	db := testDB(t)

	id, err := CreateGame(db, "ABC123", "guild1", "chan1", "user1")
	if err != nil {
		t.Fatal(err)
	}

	game, err := GetGameByID(db, id)
	if err != nil {
		t.Fatal(err)
	}

	if game.JoinCode != "ABC123" {
		t.Errorf("expected join code ABC123, got %s", game.JoinCode)
	}
	if game.GuildID != "guild1" {
		t.Errorf("expected guild1, got %s", game.GuildID)
	}
	if game.Status != "lobby" {
		t.Errorf("expected lobby, got %s", game.Status)
	}
	if game.CurrentPhase != "lobby" {
		t.Errorf("expected lobby phase, got %s", game.CurrentPhase)
	}
}

func TestGetGameByJoinCode(t *testing.T) {
	db := testDB(t)

	CreateGame(db, "XYZ789", "guild1", "chan1", "user1")

	game, err := GetGameByJoinCode(db, "XYZ789")
	if err != nil {
		t.Fatal(err)
	}
	if game.JoinCode != "XYZ789" {
		t.Errorf("expected XYZ789, got %s", game.JoinCode)
	}
}

func TestGetGameByChannel(t *testing.T) {
	db := testDB(t)

	CreateGame(db, "CH1234", "guild1", "chan42", "user1")

	game, err := GetGameByChannel(db, "chan42")
	if err != nil {
		t.Fatal(err)
	}
	if game.ChannelID != "chan42" {
		t.Errorf("expected chan42, got %s", game.ChannelID)
	}
}

func TestUpdateGameStatus(t *testing.T) {
	db := testDB(t)

	id, _ := CreateGame(db, "ST1234", "guild1", "chan1", "user1")

	if err := UpdateGameStatus(db, id, "active", "competition"); err != nil {
		t.Fatal(err)
	}

	game, _ := GetGameByID(db, id)
	if game.Status != "active" {
		t.Errorf("expected active, got %s", game.Status)
	}
	if game.CurrentPhase != "competition" {
		t.Errorf("expected competition, got %s", game.CurrentPhase)
	}
}

func TestUpdateGameTimers(t *testing.T) {
	db := testDB(t)

	id, _ := CreateGame(db, "TM1234", "guild1", "chan1", "user1")

	if err := UpdateGameTimers(db, id, 10, 20, 30, 40); err != nil {
		t.Fatal(err)
	}

	game, _ := GetGameByID(db, id)
	if game.TimerDiscussionMinutes != 10 {
		t.Errorf("expected discussion 10, got %d", game.TimerDiscussionMinutes)
	}
	if game.TimerVotingMinutes != 20 {
		t.Errorf("expected voting 20, got %d", game.TimerVotingMinutes)
	}
	if game.TimerNightMinutes != 30 {
		t.Errorf("expected night 30, got %d", game.TimerNightMinutes)
	}
	if game.TimerCompetitionMinutes != 40 {
		t.Errorf("expected competition 40, got %d", game.TimerCompetitionMinutes)
	}
}
