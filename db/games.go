package db

import (
	"database/sql"
	"time"
)

// Game represents a row in the games table.
type Game struct {
	ID                     int64
	JoinCode               string
	GuildID                string
	ChannelID              string
	CreatedBy              string
	Status                 string
	CurrentPhase           string
	CurrentRound           int
	TraitorThreadID        string
	TimerBreakfastMinutes  int
	TimerRoundtableMinutes int
	TimerNightMinutes      int
	TimerMissionMinutes    int
	RevealThreshold        int
	RecruitmentPending     bool
	HiatusStart            string
	HiatusEnd              string
	HiatusTimezone         string
	BuyinAmount            int
	EndBy                  string
	CreatedAt              time.Time
	UpdatedAt              time.Time
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
	var recruitmentPending int
	err := row.Scan(
		&g.ID, &g.JoinCode, &g.GuildID, &g.ChannelID, &g.CreatedBy,
		&g.Status, &g.CurrentPhase, &g.CurrentRound, &g.TraitorThreadID,
		&g.TimerBreakfastMinutes, &g.TimerRoundtableMinutes, &g.TimerNightMinutes,
		&g.TimerMissionMinutes, &g.RevealThreshold, &recruitmentPending,
		&g.HiatusStart, &g.HiatusEnd, &g.HiatusTimezone,
		&g.BuyinAmount, &g.EndBy,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	g.RecruitmentPending = recruitmentPending != 0
	return g, nil
}

const gameColumns = `id, join_code, guild_id, channel_id, created_by, status, current_phase, current_round, traitor_thread_id, timer_breakfast_minutes, timer_roundtable_minutes, timer_night_minutes, timer_mission_minutes, reveal_threshold, recruitment_pending, hiatus_start, hiatus_end, hiatus_timezone, buyin_amount, end_by, created_at, updated_at`

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
func UpdateGameTimers(db *sql.DB, gameID int64, breakfast, roundtable, night, mission int) error {
	_, err := db.Exec(
		`UPDATE games SET timer_breakfast_minutes = ?, timer_roundtable_minutes = ?, timer_night_minutes = ?, timer_mission_minutes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		breakfast, roundtable, night, mission, gameID,
	)
	return err
}

// SetRecruitmentPending sets the recruitment_pending flag for a game.
func SetRecruitmentPending(db *sql.DB, gameID int64, pending bool) error {
	val := 0
	if pending {
		val = 1
	}
	_, err := db.Exec(`UPDATE games SET recruitment_pending = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, val, gameID)
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

// UpdateGameEndBy sets the end-by deadline for a game.
func UpdateGameEndBy(db *sql.DB, gameID int64, endBy string) error {
	_, err := db.Exec(`UPDATE games SET end_by = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, endBy, gameID)
	return err
}

// UpdateGameBuyin sets the buy-in amount (in cents) for a game.
func UpdateGameBuyin(db *sql.DB, gameID int64, amountCents int) error {
	_, err := db.Exec(`UPDATE games SET buyin_amount = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, amountCents, gameID)
	return err
}

// GetFinishedGameByChannel finds the most recent finished game in the given channel.
func GetFinishedGameByChannel(db *sql.DB, channelID string) (*Game, error) {
	row := db.QueryRow(`SELECT `+gameColumns+` FROM games WHERE channel_id = ? AND status = 'finished' ORDER BY id DESC LIMIT 1`, channelID)
	return scanGame(row)
}

// FinishAllGames marks all non-finished games in a guild as finished.
// Returns the number of games affected.
func FinishAllGames(db *sql.DB, guildID string) (int64, error) {
	res, err := db.Exec(`UPDATE games SET status = 'finished', current_phase = 'finished', updated_at = CURRENT_TIMESTAMP WHERE guild_id = ? AND status != 'finished'`, guildID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// UpdateGameHiatus sets the quiet-hours configuration for a game.
func UpdateGameHiatus(db *sql.DB, gameID int64, start, end, tz string) error {
	_, err := db.Exec(
		`UPDATE games SET hiatus_start = ?, hiatus_end = ?, hiatus_timezone = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		start, end, tz, gameID,
	)
	return err
}
