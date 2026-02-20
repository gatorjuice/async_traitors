package handlers

import (
	"database/sql"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/game"
)

// HandleStartGame starts the game.
func HandleStartGame(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB, engine *game.Engine) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if i.Member.User.ID != g.CreatedBy {
		respondEphemeral(s, i, "Only the game creator can start the game.")
		return
	}

	if err := engine.StartGame(g.ID); err != nil {
		respondEphemeral(s, i, "Failed to start game: "+err.Error())
		slog.Error("start game", "error", err)
		return
	}

	respondEphemeral(s, i, "Game started! Check the channel for announcements.")
}

// HandleVote casts a banishment vote.
func HandleVote(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	if err := game.CastBanishmentVote(database, s, g.ID, g.CurrentRound, i.Member.User.ID, targetUser.ID); err != nil {
		respondEphemeral(s, i, err.Error())
		return
	}

	respondEphemeral(s, i, "Your vote has been recorded.")
}

// HandleMurderVote casts a murder vote.
func HandleMurderVote(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	if err := game.CastMurderVote(database, s, g.ID, g.CurrentRound, i.Member.User.ID, targetUser.ID); err != nil {
		respondEphemeral(s, i, err.Error())
		return
	}

	respondEphemeral(s, i, "Your murder vote has been recorded.")
}

// HandleClaimShield claims a shield (honor system).
func HandleClaimShield(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if err := game.ClaimShield(database, s, g.ID, i.Member.User.ID, g.CurrentRound); err != nil {
		respondEphemeral(s, i, "Failed to claim shield: "+err.Error())
		return
	}

	respondEphemeral(s, i, "Shield claimed!")
}

// HandleGrantShield grants a shield to a player (admin).
func HandleGrantShield(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if i.Member.User.ID != g.CreatedBy {
		respondEphemeral(s, i, "Only the game creator can grant shields.")
		return
	}

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	if err := game.AdminGrantShield(database, s, g.ID, targetUser.ID, g.CurrentRound); err != nil {
		respondEphemeral(s, i, "Failed to grant shield: "+err.Error())
		return
	}

	respondEphemeral(s, i, "Shield granted!")
}

// HandleAdvancePhase advances to the next phase (admin).
func HandleAdvancePhase(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB, engine *game.Engine) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if i.Member.User.ID != g.CreatedBy {
		respondEphemeral(s, i, "Only the game creator can advance phases.")
		return
	}

	if err := engine.AdvancePhase(g.ID); err != nil {
		respondEphemeral(s, i, "Failed to advance phase: "+err.Error())
		slog.Error("advance phase", "error", err)
		return
	}

	respondEphemeral(s, i, "Phase advanced!")
}
