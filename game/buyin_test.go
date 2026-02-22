package game

import (
	"testing"

	"github.com/gatorjuice/async_traitors/db"
)

func TestFormatCents(t *testing.T) {
	tests := []struct {
		cents int
		want  string
	}{
		{0, "$0.00"},
		{100, "$1.00"},
		{550, "$5.50"},
		{1000, "$10.00"},
		{1, "$0.01"},
		{1050, "$10.50"},
	}
	for _, tt := range tests {
		got := FormatCents(tt.cents)
		if got != tt.want {
			t.Errorf("FormatCents(%d) = %q, want %q", tt.cents, got, tt.want)
		}
	}
}

func TestCalculatePayouts_FaithfulWin(t *testing.T) {
	players := []db.Player{
		{DiscordID: "a", DiscordName: "Alice", Role: "faithful", Status: "alive"},
		{DiscordID: "b", DiscordName: "Bob", Role: "faithful", Status: "alive"},
		{DiscordID: "c", DiscordName: "Charlie", Role: "traitor", Status: "banished"},
		{DiscordID: "d", DiscordName: "Diana", Role: "faithful", Status: "murdered"},
	}

	winners, losers := CalculatePayouts(players, "faithful", 500)

	if len(winners) != 2 {
		t.Fatalf("expected 2 winners, got %d", len(winners))
	}
	if len(losers) != 2 {
		t.Fatalf("expected 2 losers, got %d", len(losers))
	}

	// Each winner receives (2 losers * $5) / 2 winners = $5.00 = 500 cents
	for _, w := range winners {
		if w.Amount != 500 {
			t.Errorf("winner %s amount = %d, want 500", w.PlayerName, w.Amount)
		}
	}

	// Each loser owes $5.00 total (500/2 = 250 per winner, * 2 winners = 500)
	for _, l := range losers {
		if l.Amount != 500 {
			t.Errorf("loser %s amount = %d, want 500", l.PlayerName, l.Amount)
		}
	}
}

func TestCalculatePayouts_TraitorsWin(t *testing.T) {
	players := []db.Player{
		{DiscordID: "a", DiscordName: "Alice", Role: "faithful", Status: "alive"},
		{DiscordID: "b", DiscordName: "Bob", Role: "traitor", Status: "alive"},
		{DiscordID: "c", DiscordName: "Charlie", Role: "traitor", Status: "alive"},
		{DiscordID: "d", DiscordName: "Diana", Role: "faithful", Status: "murdered"},
		{DiscordID: "e", DiscordName: "Eve", Role: "faithful", Status: "banished"},
	}

	winners, losers := CalculatePayouts(players, "traitors", 1000)

	if len(winners) != 2 {
		t.Fatalf("expected 2 winners, got %d", len(winners))
	}
	if len(losers) != 3 {
		t.Fatalf("expected 3 losers, got %d", len(losers))
	}

	// Each winner receives (3 losers * $10) / 2 winners = $15.00 = 1500 cents
	for _, w := range winners {
		if w.Amount != 1500 {
			t.Errorf("winner %s amount = %d, want 1500", w.PlayerName, w.Amount)
		}
	}

	// Each loser owes $10.00 total (1000/2 = 500 per winner, * 2 winners = 1000)
	for _, l := range losers {
		if l.Amount != 1000 {
			t.Errorf("loser %s amount = %d, want 1000", l.PlayerName, l.Amount)
		}
	}
}

func TestCalculatePayouts_ZeroBuyin(t *testing.T) {
	players := []db.Player{
		{DiscordID: "a", DiscordName: "Alice", Role: "faithful", Status: "alive"},
		{DiscordID: "b", DiscordName: "Bob", Role: "traitor", Status: "banished"},
	}

	winners, losers := CalculatePayouts(players, "faithful", 0)

	if len(winners) != 0 {
		t.Errorf("expected 0 winners with zero buyin, got %d", len(winners))
	}
	if len(losers) != 0 {
		t.Errorf("expected 0 losers with zero buyin, got %d", len(losers))
	}
}

func TestCalculatePayouts_RemainderStaysWithLosers(t *testing.T) {
	// 3 losers, 2 winners, $3 buy-in (300 cents)
	// Each loser owes 300 total. Per-winner: 300/2 = 150 cents each.
	// Each winner gets (3*300)/2 = 450 cents.
	players := []db.Player{
		{DiscordID: "a", DiscordName: "Alice", Role: "faithful", Status: "alive"},
		{DiscordID: "b", DiscordName: "Bob", Role: "faithful", Status: "alive"},
		{DiscordID: "c", DiscordName: "Charlie", Role: "traitor", Status: "banished"},
		{DiscordID: "d", DiscordName: "Diana", Role: "faithful", Status: "murdered"},
		{DiscordID: "e", DiscordName: "Eve", Role: "faithful", Status: "murdered"},
	}

	winners, losers := CalculatePayouts(players, "faithful", 300)

	if len(winners) != 2 {
		t.Fatalf("expected 2 winners, got %d", len(winners))
	}
	if len(losers) != 3 {
		t.Fatalf("expected 3 losers, got %d", len(losers))
	}

	for _, w := range winners {
		if w.Amount != 450 {
			t.Errorf("winner %s amount = %d, want 450", w.PlayerName, w.Amount)
		}
	}

	// Each loser: 150 * 2 = 300
	for _, l := range losers {
		if l.Amount != 300 {
			t.Errorf("loser %s amount = %d, want 300", l.PlayerName, l.Amount)
		}
	}
}
