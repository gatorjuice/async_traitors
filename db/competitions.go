package db

import (
	"database/sql"
	"errors"
	"time"
)

// Competition represents a row in the competitions table.
type Competition struct {
	ID           int64
	GameID       int64
	Round        int
	CompType     string
	QuestionData string
	Answer       string
	Status       string
	CreatedAt    time.Time
}

// CompetitionResult represents a row in the competition_results table.
type CompetitionResult struct {
	ID              int64
	CompetitionID   int64
	PlayerDiscordID string
	Answer          string
	Correct         bool
	TimeMs          int64
	SubmittedAt     time.Time
}

// ShieldLogEntry represents a row in the shield_log table.
type ShieldLogEntry struct {
	ID              int64
	GameID          int64
	PlayerDiscordID string
	Source          string
	RoundGranted    int
	RoundUsed       *int
	CreatedAt       time.Time
}

// CreateCompetition inserts a new competition and returns its ID.
func CreateCompetition(db *sql.DB, gameID int64, round int, compType, questionData, answer string) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO competitions (game_id, round, comp_type, question_data, answer) VALUES (?, ?, ?, ?, ?)`,
		gameID, round, compType, questionData, answer,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetActiveCompetition retrieves the active competition for a game.
func GetActiveCompetition(db *sql.DB, gameID int64) (*Competition, error) {
	c := &Competition{}
	err := db.QueryRow(
		`SELECT id, game_id, round, comp_type, question_data, answer, status, created_at FROM competitions WHERE game_id = ? AND status = 'active' LIMIT 1`,
		gameID,
	).Scan(&c.ID, &c.GameID, &c.Round, &c.CompType, &c.QuestionData, &c.Answer, &c.Status, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// SubmitCompetitionResult records a player's competition answer.
func SubmitCompetitionResult(db *sql.DB, competitionID int64, playerDiscordID, answer string, correct bool, timeMs int64) error {
	correctInt := 0
	if correct {
		correctInt = 1
	}
	_, err := db.Exec(
		`INSERT INTO competition_results (competition_id, player_discord_id, answer, correct, time_ms) VALUES (?, ?, ?, ?, ?)`,
		competitionID, playerDiscordID, answer, correctInt, timeMs,
	)
	return err
}

// GetCompetitionResults retrieves all results for a competition.
func GetCompetitionResults(db *sql.DB, competitionID int64) ([]CompetitionResult, error) {
	rows, err := db.Query(
		`SELECT id, competition_id, player_discord_id, answer, correct, time_ms, submitted_at FROM competition_results WHERE competition_id = ? ORDER BY submitted_at ASC`,
		competitionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CompetitionResult
	for rows.Next() {
		var r CompetitionResult
		var correctInt int
		if err := rows.Scan(&r.ID, &r.CompetitionID, &r.PlayerDiscordID, &r.Answer, &correctInt, &r.TimeMs, &r.SubmittedAt); err != nil {
			return nil, err
		}
		r.Correct = correctInt != 0
		results = append(results, r)
	}
	return results, rows.Err()
}

// EndCompetition marks a competition as completed.
func EndCompetition(db *sql.DB, competitionID int64) error {
	_, err := db.Exec(`UPDATE competitions SET status = 'completed' WHERE id = ?`, competitionID)
	return err
}

// GrantShield inserts a shield log entry and sets the player's shield flag.
func GrantShield(db *sql.DB, gameID int64, playerDiscordID, source string, round int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO shield_log (game_id, player_discord_id, source, round_granted) VALUES (?, ?, ?, ?)`,
		gameID, playerDiscordID, source, round,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		`UPDATE players SET has_shield = 1 WHERE game_id = ? AND discord_id = ?`,
		gameID, playerDiscordID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ConsumeShield removes a player's shield in a transaction.
func ConsumeShield(db *sql.DB, gameID int64, playerDiscordID string, round int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var hasShield int
	err = tx.QueryRow(`SELECT has_shield FROM players WHERE game_id = ? AND discord_id = ?`, gameID, playerDiscordID).Scan(&hasShield)
	if err != nil {
		return err
	}
	if hasShield == 0 {
		return errors.New("player does not have a shield")
	}

	_, err = tx.Exec(`UPDATE players SET has_shield = 0 WHERE game_id = ? AND discord_id = ?`, gameID, playerDiscordID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		`UPDATE shield_log SET round_used = ? WHERE game_id = ? AND player_discord_id = ? AND round_used IS NULL`,
		round, gameID, playerDiscordID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetShieldLog retrieves all shield log entries for a game.
func GetShieldLog(db *sql.DB, gameID int64) ([]ShieldLogEntry, error) {
	rows, err := db.Query(
		`SELECT id, game_id, player_discord_id, source, round_granted, round_used, created_at FROM shield_log WHERE game_id = ?`,
		gameID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ShieldLogEntry
	for rows.Next() {
		var e ShieldLogEntry
		if err := rows.Scan(&e.ID, &e.GameID, &e.PlayerDiscordID, &e.Source, &e.RoundGranted, &e.RoundUsed, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
