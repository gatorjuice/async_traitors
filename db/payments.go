package db

import (
	"database/sql"
	"time"
)

// Payment represents a row in the payments table.
type Payment struct {
	ID              int64
	GameID          int64
	WinnerDiscordID string
	LoserDiscordID  string
	CreatedAt       time.Time
}

// MarkPaid records that a loser has paid a winner. Idempotent (INSERT OR IGNORE).
func MarkPaid(db *sql.DB, gameID int64, winnerDiscordID, loserDiscordID string) error {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO payments (game_id, winner_discord_id, loser_discord_id) VALUES (?, ?, ?)`,
		gameID, winnerDiscordID, loserDiscordID,
	)
	return err
}

// IsMarkedPaid checks whether a specific winner-loser payment has been confirmed.
func IsMarkedPaid(db *sql.DB, gameID int64, winnerDiscordID, loserDiscordID string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM payments WHERE game_id = ? AND winner_discord_id = ? AND loser_discord_id = ?`,
		gameID, winnerDiscordID, loserDiscordID,
	).Scan(&count)
	return count > 0, err
}

// GetPaymentsByWinner returns all payment records for a given winner in a game.
func GetPaymentsByWinner(db *sql.DB, gameID int64, winnerDiscordID string) ([]Payment, error) {
	rows, err := db.Query(
		`SELECT id, game_id, winner_discord_id, loser_discord_id, created_at FROM payments WHERE game_id = ? AND winner_discord_id = ?`,
		gameID, winnerDiscordID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPayments(rows)
}

// GetPaymentsByLoser returns all payment records for a given loser in a game.
func GetPaymentsByLoser(db *sql.DB, gameID int64, loserDiscordID string) ([]Payment, error) {
	rows, err := db.Query(
		`SELECT id, game_id, winner_discord_id, loser_discord_id, created_at FROM payments WHERE game_id = ? AND loser_discord_id = ?`,
		gameID, loserDiscordID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPayments(rows)
}

func scanPayments(rows *sql.Rows) ([]Payment, error) {
	var payments []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.ID, &p.GameID, &p.WinnerDiscordID, &p.LoserDiscordID, &p.CreatedAt); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}
