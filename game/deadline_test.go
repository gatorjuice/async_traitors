package game

import (
	"testing"
	"time"
)

func TestEstimateRounds(t *testing.T) {
	tests := []struct {
		players  int
		expected int
	}{
		{4, 3},
		{6, 4},
		{8, 5},
		{10, 6},
		{12, 7},
		{3, 2},  // floor
		{2, 2},  // floor
		{20, 11},
	}
	for _, tt := range tests {
		got := EstimateRounds(tt.players)
		if got != tt.expected {
			t.Errorf("EstimateRounds(%d) = %d, want %d", tt.players, got, tt.expected)
		}
	}
}

func TestAvailableActiveMinutes_NoHiatus(t *testing.T) {
	from := time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 22, 14, 0, 0, 0, time.UTC) // 4 hours = 240 minutes

	got := AvailableActiveMinutes(from, to, "", "", "UTC")
	if got != 240 {
		t.Errorf("no hiatus: got %d, want 240", got)
	}
}

func TestAvailableActiveMinutes_WithHiatus(t *testing.T) {
	// 24 hours, with a 9-hour hiatus window (22:00-07:00).
	// Active time per day: 15 hours = 900 minutes.
	from := time.Date(2026, 2, 22, 7, 0, 0, 0, time.UTC) // start at hiatus end
	to := time.Date(2026, 2, 23, 7, 0, 0, 0, time.UTC)   // 24 wall hours later

	got := AvailableActiveMinutes(from, to, "22:00", "07:00", "UTC")
	// 07:00-22:00 = 15 hours = 900 minutes of active time
	if got != 900 {
		t.Errorf("with hiatus: got %d, want 900", got)
	}
}

func TestAvailableActiveMinutes_StartsDuringHiatus(t *testing.T) {
	// Start at midnight (during 22:00-07:00 hiatus)
	from := time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC) // noon

	got := AvailableActiveMinutes(from, to, "22:00", "07:00", "UTC")
	// Hiatus 00:00-07:00 → skipped. Active 07:00-12:00 = 300 min.
	if got != 300 {
		t.Errorf("starts during hiatus: got %d, want 300", got)
	}
}

func TestAvailableActiveMinutes_ToBeforeFrom(t *testing.T) {
	from := time.Date(2026, 2, 22, 14, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC)

	got := AvailableActiveMinutes(from, to, "", "", "UTC")
	if got != 0 {
		t.Errorf("to before from: got %d, want 0", got)
	}
}

func TestCalculateTimersFromDeadline_Normal(t *testing.T) {
	// 8 players, 48 hours, no hiatus → 2880 minutes
	// EstimateRounds(8) = 5
	// minutesPerRound = 2880/5 = 576
	// breakfast = 576*8/17 = 271, roundtable = 576*4/17 = 135, night = 135, mission = 576*1/17 = 33
	now := time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)

	result := CalculateTimersFromDeadline(now, deadline, 8, "", "", "UTC")

	if result.IsTooTight {
		t.Error("should not be too tight")
	}
	if result.IsTight {
		t.Error("should not be tight")
	}
	if result.BreakfastMinutes < floorBreakfast {
		t.Errorf("breakfast %d below floor %d", result.BreakfastMinutes, floorBreakfast)
	}
	if result.RoundtableMinutes < floorRoundtable {
		t.Errorf("roundtable %d below floor %d", result.RoundtableMinutes, floorRoundtable)
	}
	if result.NightMinutes < floorNight {
		t.Errorf("night %d below floor %d", result.NightMinutes, floorNight)
	}
	if result.MissionMinutes < floorMission {
		t.Errorf("mission %d below floor %d", result.MissionMinutes, floorMission)
	}
}

func TestCalculateTimersFromDeadline_TooTight(t *testing.T) {
	// 8 players, 1 hour → 60 minutes
	// EstimateRounds(8) = 5
	// floorSum = 45, 5*45 = 225 > 60 → too tight
	now := time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, 2, 22, 11, 0, 0, 0, time.UTC)

	result := CalculateTimersFromDeadline(now, deadline, 8, "", "", "UTC")

	if !result.IsTooTight {
		t.Error("should be too tight")
	}
}

func TestCalculateTimersFromDeadline_Tight(t *testing.T) {
	// 4 players, 3 hours → 180 minutes
	// EstimateRounds(4) = 3
	// minutesPerRound = 180/3 = 60
	// breakfast = 60*8/17 = 28, roundtable = 60*4/17 = 14 (< floor 15), night = 14 (>= floor 10), mission = 60*1/17 = 3 (< floor 5)
	now := time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, 2, 22, 13, 0, 0, 0, time.UTC)

	result := CalculateTimersFromDeadline(now, deadline, 4, "", "", "UTC")

	if result.IsTooTight {
		t.Error("should not be too tight (3*45=135 <= 180)")
	}
	if !result.IsTight {
		t.Error("should be tight")
	}
}

func TestCalculateTimersFromDeadline_WithHiatus(t *testing.T) {
	// 8 players, 48 wall hours but 9h hiatus per day (22:00-07:00)
	// Active per day: 15h = 900 min, 2 days = ~1800 active min
	now := time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)

	result := CalculateTimersFromDeadline(now, deadline, 8, "22:00", "07:00", "UTC")

	if result.IsTooTight {
		t.Error("should not be too tight")
	}
	// With hiatus, timers should be shorter than without
	noHiatus := CalculateTimersFromDeadline(now, deadline, 8, "", "", "UTC")
	if result.BreakfastMinutes >= noHiatus.BreakfastMinutes {
		t.Errorf("with hiatus breakfast (%d) should be less than without (%d)", result.BreakfastMinutes, noHiatus.BreakfastMinutes)
	}
}
