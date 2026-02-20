package handlers

import (
	"database/sql"

	"github.com/bwmarrin/discordgo"
)

// HandleStartCompetition starts a competition round (stub).
func HandleStartCompetition(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	respondEphemeral(s, i, "Not yet implemented.")
}

// HandleSubmitAnswer submits a competition answer (stub).
func HandleSubmitAnswer(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	respondEphemeral(s, i, "Not yet implemented.")
}

// HandleEndCompetition ends the current competition (stub).
func HandleEndCompetition(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	respondEphemeral(s, i, "Not yet implemented.")
}
