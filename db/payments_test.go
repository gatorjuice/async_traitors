package db

import "testing"

func TestMarkPaidAndIsMarkedPaid(t *testing.T) {
	db := testDB(t)
	gameID, _ := CreateGame(db, "PAY001", "g1", "c1", "u1")

	paid, err := IsMarkedPaid(db, gameID, "winner1", "loser1")
	if err != nil {
		t.Fatal(err)
	}
	if paid {
		t.Error("expected not paid before marking")
	}

	if err := MarkPaid(db, gameID, "winner1", "loser1"); err != nil {
		t.Fatal(err)
	}

	paid, err = IsMarkedPaid(db, gameID, "winner1", "loser1")
	if err != nil {
		t.Fatal(err)
	}
	if !paid {
		t.Error("expected paid after marking")
	}
}

func TestMarkPaidIdempotent(t *testing.T) {
	db := testDB(t)
	gameID, _ := CreateGame(db, "PAY002", "g1", "c1", "u1")

	if err := MarkPaid(db, gameID, "winner1", "loser1"); err != nil {
		t.Fatal(err)
	}
	// Second call should not error (INSERT OR IGNORE).
	if err := MarkPaid(db, gameID, "winner1", "loser1"); err != nil {
		t.Fatal("expected idempotent MarkPaid, got:", err)
	}

	paid, _ := IsMarkedPaid(db, gameID, "winner1", "loser1")
	if !paid {
		t.Error("expected paid after double mark")
	}
}

func TestGetPaymentsByWinner(t *testing.T) {
	db := testDB(t)
	gameID, _ := CreateGame(db, "PAY003", "g1", "c1", "u1")

	MarkPaid(db, gameID, "winner1", "loser1")
	MarkPaid(db, gameID, "winner1", "loser2")
	MarkPaid(db, gameID, "winner2", "loser1") // different winner

	payments, err := GetPaymentsByWinner(db, gameID, "winner1")
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 2 {
		t.Fatalf("expected 2 payments for winner1, got %d", len(payments))
	}

	// winner2 should have 1
	payments, err = GetPaymentsByWinner(db, gameID, "winner2")
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 1 {
		t.Fatalf("expected 1 payment for winner2, got %d", len(payments))
	}
}

func TestGetPaymentsByLoser(t *testing.T) {
	db := testDB(t)
	gameID, _ := CreateGame(db, "PAY004", "g1", "c1", "u1")

	MarkPaid(db, gameID, "winner1", "loser1")
	MarkPaid(db, gameID, "winner2", "loser1")
	MarkPaid(db, gameID, "winner1", "loser2") // different loser

	payments, err := GetPaymentsByLoser(db, gameID, "loser1")
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 2 {
		t.Fatalf("expected 2 payments for loser1, got %d", len(payments))
	}

	payments, err = GetPaymentsByLoser(db, gameID, "loser2")
	if err != nil {
		t.Fatal(err)
	}
	if len(payments) != 1 {
		t.Fatalf("expected 1 payment for loser2, got %d", len(payments))
	}
}
