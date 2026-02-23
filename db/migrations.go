package db

import "database/sql"

// RunMigrations creates all database tables.
func RunMigrations(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS games (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		join_code TEXT UNIQUE NOT NULL,
		guild_id TEXT NOT NULL,
		channel_id TEXT NOT NULL,
		created_by TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'lobby',
		current_phase TEXT NOT NULL DEFAULT 'lobby',
		current_round INTEGER NOT NULL DEFAULT 0,
		traitor_thread_id TEXT NOT NULL DEFAULT '',
		timer_breakfast_minutes INTEGER NOT NULL DEFAULT 480,
		timer_roundtable_minutes INTEGER NOT NULL DEFAULT 240,
		timer_night_minutes INTEGER NOT NULL DEFAULT 240,
		timer_mission_minutes INTEGER NOT NULL DEFAULT 60,
		reveal_threshold INTEGER NOT NULL DEFAULT 4,
		recruitment_pending INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS players (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		game_id INTEGER NOT NULL REFERENCES games(id),
		discord_id TEXT NOT NULL,
		discord_name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'unassigned',
		status TEXT NOT NULL DEFAULT 'alive',
		has_shield INTEGER NOT NULL DEFAULT 0,
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(game_id, discord_id)
	);

	CREATE TABLE IF NOT EXISTS votes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		game_id INTEGER NOT NULL REFERENCES games(id),
		round INTEGER NOT NULL,
		phase TEXT NOT NULL,
		voter_discord_id TEXT NOT NULL,
		target_discord_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(game_id, round, phase, voter_discord_id)
	);

	CREATE TABLE IF NOT EXISTS competitions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		game_id INTEGER NOT NULL REFERENCES games(id),
		round INTEGER NOT NULL,
		comp_type TEXT NOT NULL,
		question_data TEXT NOT NULL DEFAULT '',
		answer TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS competition_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		competition_id INTEGER NOT NULL REFERENCES competitions(id),
		player_discord_id TEXT NOT NULL,
		answer TEXT NOT NULL DEFAULT '',
		correct INTEGER NOT NULL DEFAULT 0,
		time_ms INTEGER NOT NULL DEFAULT 0,
		submitted_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS shield_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		game_id INTEGER NOT NULL REFERENCES games(id),
		player_discord_id TEXT NOT NULL,
		source TEXT NOT NULL,
		round_granted INTEGER NOT NULL,
		round_used INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS payments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		game_id INTEGER NOT NULL REFERENCES games(id),
		winner_discord_id TEXT NOT NULL,
		loser_discord_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(game_id, winner_discord_id, loser_discord_id)
	);`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// Idempotent ALTER TABLE migrations for new columns and renames.
	alters := []string{
		// Rename old timer columns to new names (idempotent: fails silently if already renamed).
		`ALTER TABLE games RENAME COLUMN timer_discussion_minutes TO timer_breakfast_minutes`,
		`ALTER TABLE games RENAME COLUMN timer_voting_minutes TO timer_roundtable_minutes`,
		`ALTER TABLE games RENAME COLUMN timer_competition_minutes TO timer_mission_minutes`,
		// New columns.
		`ALTER TABLE games ADD COLUMN hiatus_start TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE games ADD COLUMN hiatus_end TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE games ADD COLUMN hiatus_timezone TEXT NOT NULL DEFAULT 'UTC'`,
		`ALTER TABLE players ADD COLUMN status_round INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE players ADD COLUMN recruited_round INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE games ADD COLUMN recruitment_pending INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE games ADD COLUMN buyin_amount INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE players ADD COLUMN wallet_info TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE games ADD COLUMN end_by TEXT NOT NULL DEFAULT ''`,
	}
	for _, q := range alters {
		// Ignore "duplicate column" errors from re-running migrations.
		_, _ = db.Exec(q)
	}

	return nil
}
