package db

import "testing"

func TestCastAndGetVotes(t *testing.T) {
	db := testDB(t)
	id, _ := CreateGame(db, "VT1234", "g1", "c1", "u1")

	CastVote(db, id, 1, "roundtable", "voter1", "target1")
	CastVote(db, id, 1, "roundtable", "voter2", "target1")

	votes, err := GetVotes(db, id, 1, "roundtable")
	if err != nil {
		t.Fatal(err)
	}
	if len(votes) != 2 {
		t.Errorf("expected 2 votes, got %d", len(votes))
	}
}

func TestVoteUpsert(t *testing.T) {
	db := testDB(t)
	id, _ := CreateGame(db, "UP1234", "g1", "c1", "u1")

	CastVote(db, id, 1, "roundtable", "voter1", "target1")
	CastVote(db, id, 1, "roundtable", "voter1", "target2")

	votes, _ := GetVotes(db, id, 1, "roundtable")
	if len(votes) != 1 {
		t.Errorf("expected 1 vote after upsert, got %d", len(votes))
	}
	if votes[0].TargetDiscordID != "target2" {
		t.Errorf("expected target2, got %s", votes[0].TargetDiscordID)
	}
}

func TestClearVotes(t *testing.T) {
	db := testDB(t)
	id, _ := CreateGame(db, "CL1234", "g1", "c1", "u1")

	CastVote(db, id, 1, "roundtable", "v1", "t1")
	CastVote(db, id, 1, "roundtable", "v2", "t1")

	ClearVotes(db, id, 1, "roundtable")

	votes, _ := GetVotes(db, id, 1, "roundtable")
	if len(votes) != 0 {
		t.Errorf("expected 0 votes after clear, got %d", len(votes))
	}
}

func TestCountVotes(t *testing.T) {
	db := testDB(t)
	id, _ := CreateGame(db, "CV1234", "g1", "c1", "u1")

	CastVote(db, id, 1, "roundtable", "v1", "t1")
	CastVote(db, id, 1, "roundtable", "v2", "t1")
	CastVote(db, id, 1, "roundtable", "v3", "t2")

	count, err := CountVotes(db, id, 1, "roundtable")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}
