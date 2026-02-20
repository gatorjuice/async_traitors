package game

import "testing"

func TestValidTransitions(t *testing.T) {
	valid := []struct{ from, to Phase }{
		{PhaseLobby, PhaseCompetition},
		{PhaseLobby, PhaseDiscussion},
		{PhaseCompetition, PhaseDiscussion},
		{PhaseDiscussion, PhaseVoting},
		{PhaseVoting, PhaseNight},
		{PhaseNight, PhaseCompetition},
		{PhaseNight, PhaseDiscussion},
	}

	for _, tc := range valid {
		if !CanTransition(tc.from, tc.to) {
			t.Errorf("expected valid transition from %s to %s", tc.from, tc.to)
		}
	}
}

func TestInvalidTransitions(t *testing.T) {
	invalid := []struct{ from, to Phase }{
		{PhaseLobby, PhaseVoting},
		{PhaseLobby, PhaseNight},
		{PhaseCompetition, PhaseVoting},
		{PhaseCompetition, PhaseNight},
		{PhaseDiscussion, PhaseCompetition},
		{PhaseDiscussion, PhaseNight},
		{PhaseVoting, PhaseCompetition},
		{PhaseVoting, PhaseDiscussion},
		{PhaseNight, PhaseVoting},
	}

	for _, tc := range invalid {
		if CanTransition(tc.from, tc.to) {
			t.Errorf("expected invalid transition from %s to %s", tc.from, tc.to)
		}
	}
}
