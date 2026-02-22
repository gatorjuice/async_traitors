package game

import "testing"

func TestValidTransitions(t *testing.T) {
	valid := []struct{ from, to Phase }{
		{PhaseLobby, PhaseBreakfast},
		{PhaseBreakfast, PhaseMission},
		{PhaseMission, PhaseRoundtable},
		{PhaseRoundtable, PhaseNight},
		{PhaseNight, PhaseBreakfast},
	}

	for _, tc := range valid {
		if !CanTransition(tc.from, tc.to) {
			t.Errorf("expected valid transition from %s to %s", tc.from, tc.to)
		}
	}
}

func TestInvalidTransitions(t *testing.T) {
	invalid := []struct{ from, to Phase }{
		{PhaseLobby, PhaseRoundtable},
		{PhaseLobby, PhaseNight},
		{PhaseLobby, PhaseMission},
		{PhaseBreakfast, PhaseRoundtable},
		{PhaseBreakfast, PhaseNight},
		{PhaseMission, PhaseBreakfast},
		{PhaseMission, PhaseNight},
		{PhaseRoundtable, PhaseBreakfast},
		{PhaseRoundtable, PhaseMission},
		{PhaseNight, PhaseMission},
		{PhaseNight, PhaseRoundtable},
	}

	for _, tc := range invalid {
		if CanTransition(tc.from, tc.to) {
			t.Errorf("expected invalid transition from %s to %s", tc.from, tc.to)
		}
	}
}
