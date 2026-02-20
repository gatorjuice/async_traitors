package handlers

import (
	"database/sql"

	"github.com/bwmarrin/discordgo"
)

// HandleStartGame starts the game (stub for Phase 2).
func HandleStartGame(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	respondEphemeral(s, i, "Not yet implemented.")
}

// HandleVote casts a banishment vote (stub for Phase 2).
func HandleVote(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	respondEphemeral(s, i, "Not yet implemented.")
}

// HandleMurderVote casts a murder vote (stub for Phase 2).
func HandleMurderVote(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	respondEphemeral(s, i, "Not yet implemented.")
}

// HandleClaimShield claims a shield (stub for Phase 3).
func HandleClaimShield(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	respondEphemeral(s, i, "Not yet implemented.")
}

// HandleGrantShield grants a shield to a player (stub for Phase 3).
func HandleGrantShield(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	respondEphemeral(s, i, "Not yet implemented.")
}

// HandleAdvancePhase advances to the next phase (stub for Phase 2).
func HandleAdvancePhase(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	respondEphemeral(s, i, "Not yet implemented.")
}
