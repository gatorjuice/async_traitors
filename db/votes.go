package db

import (
	"database/sql"
	"time"
)

// Vote represents a row in the votes table.
type Vote struct {
	ID              int64
	GameID          int64
	Round           int
	Phase           string
	VoterDiscordID  string
	TargetDiscordID string
	CreatedAt       time.Time
}

// CastVote inserts or replaces a vote (upsert on unique constraint).
func CastVote(db *sql.DB, gameID int64, round int, phase, voterID, targetID string) error {
	_, err := db.Exec(
		`INSERT OR REPLACE INTO votes (game_id, round, phase, voter_discord_id, target_discord_id) VALUES (?, ?, ?, ?, ?)`,
		gameID, round, phase, voterID, targetID,
	)
	return err
}

// GetVotes returns all votes for a given round and phase.
func GetVotes(db *sql.DB, gameID int64, round int, phase string) ([]Vote, error) {
	rows, err := db.Query(
		`SELECT id, game_id, round, phase, voter_discord_id, target_discord_id, created_at FROM votes WHERE game_id = ? AND round = ? AND phase = ?`,
		gameID, round, phase,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var votes []Vote
	for rows.Next() {
		var v Vote
		if err := rows.Scan(&v.ID, &v.GameID, &v.Round, &v.Phase, &v.VoterDiscordID, &v.TargetDiscordID, &v.CreatedAt); err != nil {
			return nil, err
		}
		votes = append(votes, v)
	}
	return votes, rows.Err()
}

// CountVotes counts votes for a given round and phase.
func CountVotes(db *sql.DB, gameID int64, round int, phase string) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM votes WHERE game_id = ? AND round = ? AND phase = ?`,
		gameID, round, phase,
	).Scan(&count)
	return count, err
}

// ClearVotes deletes all votes for a given round and phase.
func ClearVotes(db *sql.DB, gameID int64, round int, phase string) error {
	_, err := db.Exec(
		`DELETE FROM votes WHERE game_id = ? AND round = ? AND phase = ?`,
		gameID, round, phase,
	)
	return err
}
