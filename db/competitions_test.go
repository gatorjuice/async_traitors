package db

import "testing"

func TestCreateAndGetCompetition(t *testing.T) {
	db := testDB(t)
	gameID, _ := CreateGame(db, "CO1234", "g1", "c1", "u1")

	compID, err := CreateCompetition(db, gameID, 1, "trivia", `{"q":"test"}`, "answer")
	if err != nil {
		t.Fatal(err)
	}

	comp, err := GetActiveCompetition(db, gameID)
	if err != nil {
		t.Fatal(err)
	}
	if comp.ID != compID {
		t.Errorf("expected competition ID %d, got %d", compID, comp.ID)
	}
	if comp.CompType != "trivia" {
		t.Errorf("expected trivia, got %s", comp.CompType)
	}
}

func TestSubmitResults(t *testing.T) {
	db := testDB(t)
	gameID, _ := CreateGame(db, "SR1234", "g1", "c1", "u1")
	compID, _ := CreateCompetition(db, gameID, 1, "trivia", `{}`, "42")

	if err := SubmitCompetitionResult(db, compID, "p1", "42", true, 1500); err != nil {
		t.Fatal(err)
	}
	if err := SubmitCompetitionResult(db, compID, "p2", "wrong", false, 2000); err != nil {
		t.Fatal(err)
	}

	results, err := GetCompetitionResults(db, compID)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if !results[0].Correct {
		t.Error("expected first result to be correct")
	}
	if results[1].Correct {
		t.Error("expected second result to be incorrect")
	}
}

func TestEndCompetition(t *testing.T) {
	db := testDB(t)
	gameID, _ := CreateGame(db, "EC1234", "g1", "c1", "u1")
	compID, _ := CreateCompetition(db, gameID, 1, "trivia", `{}`, "a")

	if err := EndCompetition(db, compID); err != nil {
		t.Fatal(err)
	}

	_, err := GetActiveCompetition(db, gameID)
	if err == nil {
		t.Error("expected no active competition after ending")
	}
}

func TestShieldLog(t *testing.T) {
	db := testDB(t)
	gameID, _ := CreateGame(db, "SL1234", "g1", "c1", "u1")
	AddPlayer(db, gameID, "p1", "Alice")

	if err := GrantShield(db, gameID, "p1", "mission", 1); err != nil {
		t.Fatal(err)
	}

	p, _ := GetPlayer(db, gameID, "p1")
	if !p.HasShield {
		t.Error("expected player to have shield after grant")
	}

	if err := ConsumeShield(db, gameID, "p1", 2); err != nil {
		t.Fatal(err)
	}

	p, _ = GetPlayer(db, gameID, "p1")
	if p.HasShield {
		t.Error("expected player to not have shield after consume")
	}

	log, err := GetShieldLog(db, gameID)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 1 {
		t.Errorf("expected 1 shield log entry, got %d", len(log))
	}
	if log[0].RoundUsed == nil {
		t.Error("expected round_used to be set")
	}
}
