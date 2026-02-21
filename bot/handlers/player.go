package handlers

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/notify"
)

// HandleJoinGame adds a player to a game by join code.
func HandleJoinGame(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	code := i.ApplicationCommandData().Options[0].StringValue()

	game, err := db.GetGameByJoinCode(database, code)
	if err != nil {
		respondEphemeral(s, i, "Game not found. Check your join code.")
		return
	}

	if game.Status != "lobby" {
		respondEphemeral(s, i, "This game has already started.")
		return
	}

	playerID := i.Member.User.ID
	playerName := i.Member.User.Username

	if err := db.AddPlayer(database, game.ID, playerID, playerName); err != nil {
		respondEphemeral(s, i, "You may have already joined this game.")
		slog.Error("add player", "error", err)
		return
	}

	notify.SendChannel(s, game.ChannelID, fmt.Sprintf("**%s** has joined the game!", playerName))
	respondEphemeral(s, i, fmt.Sprintf("You've joined the game in <#%s>!", game.ChannelID))
}

// HandleMyRole DMs the player their role.
func HandleMyRole(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	game, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	player, err := db.GetPlayer(database, game.ID, i.Member.User.ID)
	if err != nil {
		respondEphemeral(s, i, "You are not in this game.")
		return
	}

	if player.Role == "unassigned" {
		respondEphemeral(s, i, "Roles have not been assigned yet. Wait for the game to start.")
		return
	}

	var roleMsg string
	switch player.Role {
	case "traitor":
		traitors, _ := db.GetPlayersByRole(database, game.ID, "traitor")
		var others []string
		for _, t := range traitors {
			if t.DiscordID != player.DiscordID {
				others = append(others, t.DiscordName)
			}
		}
		roleMsg = "You are a **TRAITOR**! Eliminate the faithful to win."
		if len(others) > 0 {
			roleMsg += "\nYour fellow traitors: "
			for j, name := range others {
				if j > 0 {
					roleMsg += ", "
				}
				roleMsg += name
			}
		}
	case "faithful":
		roleMsg = "You are **FAITHFUL**! Find and banish the traitors to win."
	}

	if err := notify.SendDM(s, i.Member.User.ID, roleMsg); err != nil {
		respondEphemeral(s, i, "I couldn't DM you. Please check your DM settings.")
		slog.Error("send role DM", "error", err)
		return
	}

	respondEphemeral(s, i, "Check your DMs!")
}

// HandleJoinButton handles the "Join Game" button interaction.
func HandleJoinButton(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	code := strings.TrimPrefix(i.MessageComponentData().CustomID, "join-game:")

	game, err := db.GetGameByJoinCode(database, code)
	if err != nil {
		respondEphemeral(s, i, "Game not found. The join code may have expired.")
		return
	}

	if game.Status != "lobby" {
		respondEphemeral(s, i, "This game has already started.")
		return
	}

	playerID := i.Member.User.ID
	playerName := i.Member.User.Username

	if err := db.AddPlayer(database, game.ID, playerID, playerName); err != nil {
		respondEphemeral(s, i, "You may have already joined this game.")
		slog.Error("add player via button", "error", err)
		return
	}

	notify.SendChannel(s, game.ChannelID, fmt.Sprintf("**%s** has joined the game!", playerName))
	respondEphemeral(s, i, fmt.Sprintf("You've joined the game in <#%s>!", game.ChannelID))
}
