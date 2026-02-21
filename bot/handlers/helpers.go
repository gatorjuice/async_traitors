package handlers

import (
	"database/sql"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
)

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		slog.Error("respond ephemeral", "error", err)
	}
}

// requirePlayer looks up the game in the channel and verifies the caller is a participant.
// Returns nil, nil and sends an ephemeral error if either lookup fails.
func requirePlayer(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) (*db.Game, *db.Player) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return nil, nil
	}

	p, err := db.GetPlayer(database, g.ID, i.Member.User.ID)
	if err != nil {
		respondEphemeral(s, i, "You are not a player in this game.")
		return nil, nil
	}

	return g, p
}
