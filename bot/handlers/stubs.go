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
		slog.Error("start game: game lookup failed", "error", err, "channel_id", i.ChannelID)
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
func HandleVote(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB, engine *game.Engine) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("vote: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	allVoted, err := game.CastBanishmentVote(database, s, g.ID, g.CurrentRound, i.Member.User.ID, targetUser.ID)
	if err != nil {
		respondEphemeral(s, i, err.Error())
		return
	}

	respondEphemeral(s, i, "Your vote has been recorded.")

	if allVoted {
		if err := engine.AdvancePhase(g.ID); err != nil {
			slog.Error("auto-advance after all votes", "error", err, "game", g.ID)
		}
	}
}

// HandleMurderVote casts a murder vote.
func HandleMurderVote(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB, engine *game.Engine) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("murder vote: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	allVoted, err := game.CastMurderVote(database, s, g.ID, g.CurrentRound, i.Member.User.ID, targetUser.ID)
	if err != nil {
		respondEphemeral(s, i, err.Error())
		return
	}

	respondEphemeral(s, i, "Your murder vote has been recorded.")

	if allVoted {
		if err := engine.AdvancePhase(g.ID); err != nil {
			slog.Error("auto-advance after all murder votes", "error", err, "game", g.ID)
		}
	}
}

// HandleClaimShield claims a shield (honor system).
func HandleClaimShield(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	g, _ := requirePlayer(s, i, database)
	if g == nil {
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
		slog.Error("grant shield: game lookup failed", "error", err, "channel_id", i.ChannelID)
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

// HandleRecruit handles a traitor's recruitment vote.
func HandleRecruit(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB, engine *game.Engine) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("recruit: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	allVoted, err := game.RecruitPlayer(database, s, g.ID, g.CurrentRound, i.Member.User.ID, targetUser.ID)
	if err != nil {
		respondEphemeral(s, i, err.Error())
		return
	}

	respondEphemeral(s, i, "Your recruitment vote has been recorded.")

	if allVoted {
		if err := game.ResolveRecruitment(database, s, g.ID, g.CurrentRound); err != nil {
			slog.Error("resolve recruitment", "error", err, "game", g.ID)
		}
	}
}

// HandleAcceptRecruitment handles accepting a recruitment offer.
func HandleAcceptRecruitment(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB, engine *game.Engine) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("accept recruitment: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if err := game.AcceptRecruitment(engine, g.ID, i.Member.User.ID); err != nil {
		respondEphemeral(s, i, err.Error())
		return
	}

	respondEphemeral(s, i, "You have joined the traitors.")
}

// HandleRefuseRecruitment handles refusing a recruitment offer.
func HandleRefuseRecruitment(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB, engine *game.Engine) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("refuse recruitment: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if err := game.RefuseRecruitment(engine, g.ID, i.Member.User.ID); err != nil {
		respondEphemeral(s, i, err.Error())
		return
	}

	respondEphemeral(s, i, "You have refused the traitors' offer.")
}

// HandleForceRecruit force-recruits a player as a traitor (admin).
func HandleForceRecruit(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB, engine *game.Engine) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("force recruit: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if i.Member.User.ID != g.CreatedBy {
		respondEphemeral(s, i, "Only the game creator can force-recruit.")
		return
	}

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	if err := game.ForceRecruit(engine, g.ID, targetUser.ID); err != nil {
		respondEphemeral(s, i, "Failed to recruit: "+err.Error())
		return
	}

	respondEphemeral(s, i, "Player has been recruited as a traitor.")
}

// HandleAdvancePhase advances to the next phase (admin).
func HandleAdvancePhase(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB, engine *game.Engine) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("advance phase: game lookup failed", "error", err, "channel_id", i.ChannelID)
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
