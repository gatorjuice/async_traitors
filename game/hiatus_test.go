package game

import (
	"testing"
	"time"
)

func TestIsInHiatus_EmptyConfig(t *testing.T) {
	now := time.Now()
	if IsInHiatus("", "", "UTC", now) {
		t.Error("expected false for empty config")
	}
}

func TestIsInHiatus_InsideWindow(t *testing.T) {
	// 22:00 - 07:00 UTC, check at 23:00 UTC
	now := time.Date(2025, 1, 1, 23, 0, 0, 0, time.UTC)
	if !IsInHiatus("22:00", "07:00", "UTC", now) {
		t.Error("expected inside hiatus at 23:00")
	}
}

func TestIsInHiatus_OutsideWindow(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	if IsInHiatus("22:00", "07:00", "UTC", now) {
		t.Error("expected outside hiatus at 12:00")
	}
}

func TestIsInHiatus_MidnightWrap_EarlyMorning(t *testing.T) {
	// 22:00 - 07:00, check at 03:00
	now := time.Date(2025, 1, 2, 3, 0, 0, 0, time.UTC)
	if !IsInHiatus("22:00", "07:00", "UTC", now) {
		t.Error("expected inside hiatus at 03:00 (midnight wrap)")
	}
}

func TestIsInHiatus_NoWrap(t *testing.T) {
	// 02:00 - 07:00, check at 05:00
	now := time.Date(2025, 1, 1, 5, 0, 0, 0, time.UTC)
	if !IsInHiatus("02:00", "07:00", "UTC", now) {
		t.Error("expected inside hiatus at 05:00")
	}
}

func TestIsInHiatus_NoWrap_Outside(t *testing.T) {
	now := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)
	if IsInHiatus("02:00", "07:00", "UTC", now) {
		t.Error("expected outside hiatus at 08:00")
	}
}

func TestIsInHiatus_Timezone(t *testing.T) {
	// 22:00-07:00 America/New_York, check at 04:00 UTC (= 23:00 EST)
	now := time.Date(2025, 1, 1, 4, 0, 0, 0, time.UTC)
	if !IsInHiatus("22:00", "07:00", "America/New_York", now) {
		t.Error("expected inside hiatus (04:00 UTC = 23:00 EST)")
	}
}

func TestTimeUntilHiatusEnd_NotInHiatus(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	d := TimeUntilHiatusEnd("22:00", "07:00", "UTC", now)
	if d != 0 {
		t.Errorf("expected 0, got %v", d)
	}
}

func TestTimeUntilHiatusEnd_InHiatus(t *testing.T) {
	// 22:00-07:00, check at 23:00 -> 8 hours until end
	now := time.Date(2025, 1, 1, 23, 0, 0, 0, time.UTC)
	d := TimeUntilHiatusEnd("22:00", "07:00", "UTC", now)
	expected := 8 * time.Hour
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestTimeUntilHiatusEnd_EarlyMorning(t *testing.T) {
	// 22:00-07:00, check at 03:00 -> 4 hours until end
	now := time.Date(2025, 1, 2, 3, 0, 0, 0, time.UTC)
	d := TimeUntilHiatusEnd("22:00", "07:00", "UTC", now)
	expected := 4 * time.Hour
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestEffectiveWallDuration_NoHiatus(t *testing.T) {
	start := time.Now()
	d := EffectiveWallDuration(start, 30*time.Minute, "", "", "UTC")
	if d != 30*time.Minute {
		t.Errorf("expected 30m, got %v", d)
	}
}

func TestEffectiveWallDuration_SpansHiatus(t *testing.T) {
	// Start at 21:00 UTC, need 2h active time, hiatus 22:00-07:00
	// Active: 21:00-22:00 (1h), then hiatus 22:00-07:00 (9h), then 07:00-08:00 (1h)
	// Total wall: 11 hours
	start := time.Date(2025, 1, 1, 21, 0, 0, 0, time.UTC)
	d := EffectiveWallDuration(start, 2*time.Hour, "22:00", "07:00", "UTC")
	expected := 11 * time.Hour
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestEffectiveWallDuration_StartInsideHiatus(t *testing.T) {
	// Start at 23:00 UTC, need 1h active time, hiatus 22:00-07:00
	// Hiatus until 07:00 (8h), then 07:00-08:00 (1h active)
	// Total wall: 9 hours
	start := time.Date(2025, 1, 1, 23, 0, 0, 0, time.UTC)
	d := EffectiveWallDuration(start, 1*time.Hour, "22:00", "07:00", "UTC")
	expected := 9 * time.Hour
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestEffectiveWallDuration_FitsBeforeHiatus(t *testing.T) {
	// Start at 20:00 UTC, need 1h active time, hiatus 22:00-07:00
	// Active: 20:00-21:00 — all fits before hiatus
	start := time.Date(2025, 1, 1, 20, 0, 0, 0, time.UTC)
	d := EffectiveWallDuration(start, 1*time.Hour, "22:00", "07:00", "UTC")
	expected := 1 * time.Hour
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}
