package db

import (
	"database/sql"
	"time"
)

// Game represents a row in the games table.
type Game struct {
	ID                      int64
	JoinCode                string
	GuildID                 string
	ChannelID               string
	CreatedBy               string
	Status                  string
	CurrentPhase            string
	CurrentRound            int
	TraitorThreadID         string
	TimerDiscussionMinutes  int
	TimerVotingMinutes      int
	TimerNightMinutes       int
	TimerCompetitionMinutes int
	RevealThreshold         int
	HiatusStart             string
	HiatusEnd               string
	HiatusTimezone          string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// CreateGame inserts a new game and returns its ID.
func CreateGame(db *sql.DB, joinCode, guildID, channelID, createdBy string) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO games (join_code, guild_id, channel_id, created_by) VALUES (?, ?, ?, ?)`,
		joinCode, guildID, channelID, createdBy,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func scanGame(row interface{ Scan(...any) error }) (*Game, error) {
	g := &Game{}
	err := row.Scan(
		&g.ID, &g.JoinCode, &g.GuildID, &g.ChannelID, &g.CreatedBy,
		&g.Status, &g.CurrentPhase, &g.CurrentRound, &g.TraitorThreadID,
		&g.TimerDiscussionMinutes, &g.TimerVotingMinutes, &g.TimerNightMinutes,
		&g.TimerCompetitionMinutes, &g.RevealThreshold,
		&g.HiatusStart, &g.HiatusEnd, &g.HiatusTimezone,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return g, nil
}

const gameColumns = `id, join_code, guild_id, channel_id, created_by, status, current_phase, current_round, traitor_thread_id, timer_discussion_minutes, timer_voting_minutes, timer_night_minutes, timer_competition_minutes, reveal_threshold, hiatus_start, hiatus_end, hiatus_timezone, created_at, updated_at`

// GetGameByJoinCode retrieves a game by its join code.
func GetGameByJoinCode(db *sql.DB, joinCode string) (*Game, error) {
	row := db.QueryRow(`SELECT `+gameColumns+` FROM games WHERE join_code = ?`, joinCode)
	return scanGame(row)
}

// GetGameByID retrieves a game by its ID.
func GetGameByID(db *sql.DB, gameID int64) (*Game, error) {
	row := db.QueryRow(`SELECT `+gameColumns+` FROM games WHERE id = ?`, gameID)
	return scanGame(row)
}

// GetGameByChannel finds an active game in the given channel.
func GetGameByChannel(db *sql.DB, channelID string) (*Game, error) {
	row := db.QueryRow(`SELECT `+gameColumns+` FROM games WHERE channel_id = ? AND status != 'finished' ORDER BY id DESC LIMIT 1`, channelID)
	return scanGame(row)
}

// UpdateGameStatus updates a game's status and phase.
func UpdateGameStatus(db *sql.DB, gameID int64, status, phase string) error {
	_, err := db.Exec(`UPDATE games SET status = ?, current_phase = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, phase, gameID)
	return err
}

// UpdateGameRound updates a game's current round.
func UpdateGameRound(db *sql.DB, gameID int64, round int) error {
	_, err := db.Exec(`UPDATE games SET current_round = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, round, gameID)
	return err
}

// UpdateGameTimers updates the timer settings for a game.
func UpdateGameTimers(db *sql.DB, gameID int64, discussion, voting, night, competition int) error {
	_, err := db.Exec(
		`UPDATE games SET timer_discussion_minutes = ?, timer_voting_minutes = ?, timer_night_minutes = ?, timer_competition_minutes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		discussion, voting, night, competition, gameID,
	)
	return err
}

// SetTraitorThreadID stores the traitor thread ID for a game.
func SetTraitorThreadID(db *sql.DB, gameID int64, threadID string) error {
	_, err := db.Exec(`UPDATE games SET traitor_thread_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, threadID, gameID)
	return err
}

// UpdateGamePhase updates a game's current phase.
func UpdateGamePhase(db *sql.DB, gameID int64, phase string) error {
	_, err := db.Exec(`UPDATE games SET current_phase = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, phase, gameID)
	return err
}

// UpdateGameHiatus sets the quiet-hours configuration for a game.
func UpdateGameHiatus(db *sql.DB, gameID int64, start, end, tz string) error {
	_, err := db.Exec(
		`UPDATE games SET hiatus_start = ?, hiatus_end = ?, hiatus_timezone = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		start, end, tz, gameID,
	)
	return err
}
