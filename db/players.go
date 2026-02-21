package db

import (
	"database/sql"
	"time"
)

// Player represents a row in the players table.
type Player struct {
	ID          int64
	GameID      int64
	DiscordID   string
	DiscordName string
	Role        string
	Status      string
	HasShield   bool
	StatusRound int
	JoinedAt    time.Time
}

func scanPlayer(row interface{ Scan(...any) error }) (*Player, error) {
	p := &Player{}
	var hasShield int
	err := row.Scan(&p.ID, &p.GameID, &p.DiscordID, &p.DiscordName, &p.Role, &p.Status, &hasShield, &p.StatusRound, &p.JoinedAt)
	if err != nil {
		return nil, err
	}
	p.HasShield = hasShield != 0
	return p, nil
}

func scanPlayers(rows *sql.Rows) ([]Player, error) {
	var players []Player
	for rows.Next() {
		p, err := scanPlayer(rows)
		if err != nil {
			return nil, err
		}
		players = append(players, *p)
	}
	return players, rows.Err()
}

const playerColumns = `id, game_id, discord_id, discord_name, role, status, has_shield, status_round, joined_at`

// AddPlayer adds a player to a game.
func AddPlayer(db *sql.DB, gameID int64, discordID, discordName string) error {
	_, err := db.Exec(`INSERT INTO players (game_id, discord_id, discord_name) VALUES (?, ?, ?)`, gameID, discordID, discordName)
	return err
}

// GetPlayer retrieves a player by game and discord ID.
func GetPlayer(db *sql.DB, gameID int64, discordID string) (*Player, error) {
	row := db.QueryRow(`SELECT `+playerColumns+` FROM players WHERE game_id = ? AND discord_id = ?`, gameID, discordID)
	return scanPlayer(row)
}

// GetAlivePlayers returns all alive players in a game.
func GetAlivePlayers(db *sql.DB, gameID int64) ([]Player, error) {
	rows, err := db.Query(`SELECT `+playerColumns+` FROM players WHERE game_id = ? AND status = 'alive'`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlayers(rows)
}

// GetPlayersByRole returns alive players with the given role.
func GetPlayersByRole(db *sql.DB, gameID int64, role string) ([]Player, error) {
	rows, err := db.Query(`SELECT `+playerColumns+` FROM players WHERE game_id = ? AND role = ? AND status = 'alive'`, gameID, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlayers(rows)
}

// GetAllPlayers returns all players in a game regardless of status.
func GetAllPlayers(db *sql.DB, gameID int64) ([]Player, error) {
	rows, err := db.Query(`SELECT `+playerColumns+` FROM players WHERE game_id = ?`, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlayers(rows)
}

// UpdatePlayerRole sets a player's role.
func UpdatePlayerRole(db *sql.DB, gameID int64, discordID, role string) error {
	_, err := db.Exec(`UPDATE players SET role = ? WHERE game_id = ? AND discord_id = ?`, role, gameID, discordID)
	return err
}

// UpdatePlayerStatus sets a player's status.
func UpdatePlayerStatus(db *sql.DB, gameID int64, discordID, status string) error {
	_, err := db.Exec(`UPDATE players SET status = ? WHERE game_id = ? AND discord_id = ?`, status, gameID, discordID)
	return err
}

// UpdatePlayerShield sets a player's shield status.
func UpdatePlayerShield(db *sql.DB, gameID int64, discordID string, hasShield bool) error {
	val := 0
	if hasShield {
		val = 1
	}
	_, err := db.Exec(`UPDATE players SET has_shield = ? WHERE game_id = ? AND discord_id = ?`, val, gameID, discordID)
	return err
}

// CountPlayersByStatus counts players with a given status in a game.
func CountPlayersByStatus(db *sql.DB, gameID int64, status string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM players WHERE game_id = ? AND status = ?`, gameID, status).Scan(&count)
	return count, err
}

// UpdatePlayerStatusWithRound sets a player's status and records which round the status change occurred.
func UpdatePlayerStatusWithRound(db *sql.DB, gameID int64, discordID, status string, round int) error {
	_, err := db.Exec(`UPDATE players SET status = ?, status_round = ? WHERE game_id = ? AND discord_id = ?`, status, round, gameID, discordID)
	return err
}

// GetPlayersByStatusAndRound returns players with the given status who were eliminated in the given round.
func GetPlayersByStatusAndRound(db *sql.DB, gameID int64, status string, round int) ([]Player, error) {
	rows, err := db.Query(`SELECT `+playerColumns+` FROM players WHERE game_id = ? AND status = ? AND status_round = ?`, gameID, status, round)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlayers(rows)
}
