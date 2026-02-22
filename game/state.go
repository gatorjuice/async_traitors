package game

// GameStatus represents the overall status of a game.
type GameStatus string

const (
	StatusLobby    GameStatus = "lobby"
	StatusActive   GameStatus = "active"
	StatusFinished GameStatus = "finished"
)

// Phase represents the current phase within a round.
type Phase string

const (
	PhaseLobby      Phase = "lobby"
	PhaseBreakfast  Phase = "breakfast"
	PhaseMission    Phase = "mission"
	PhaseRoundtable Phase = "roundtable"
	PhaseNight      Phase = "night"
)

// Role represents a player's secret role.
type Role string

const (
	RoleUnassigned Role = "unassigned"
	RoleTraitor    Role = "traitor"
	RoleFaithful   Role = "faithful"
)

// PlayerStatus represents a player's alive/dead status.
type PlayerStatus string

const (
	PlayerAlive    PlayerStatus = "alive"
	PlayerBanished PlayerStatus = "banished"
	PlayerMurdered PlayerStatus = "murdered"
)

// ValidTransitions maps each phase to its valid next phases.
var ValidTransitions = map[Phase][]Phase{
	PhaseLobby:      {PhaseBreakfast},
	PhaseBreakfast:  {PhaseMission},
	PhaseMission:    {PhaseRoundtable},
	PhaseRoundtable: {PhaseNight},
	PhaseNight:      {PhaseBreakfast},
}

// CanTransition checks if transitioning from one phase to another is valid.
func CanTransition(from, to Phase) bool {
	targets, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}
