package game

import (
	"database/sql"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/notify"
)

// GrantShield grants a shield to a player and DMs them.
func GrantShield(database *sql.DB, s *discordgo.Session, gameID int64, playerID, source string, round int) error {
	if err := db.GrantShield(database, gameID, playerID, source, round); err != nil {
		return err
	}
	notify.SendDM(s, playerID, "You have been granted a shield! It will protect you from one murder attempt.")
	return nil
}

// ClaimShield lets a player claim a shield (honor system).
func ClaimShield(database *sql.DB, s *discordgo.Session, gameID int64, playerID string, round int) error {
	game, err := db.GetGameByID(database, gameID)
	if err != nil {
		return err
	}

	player, err := db.GetPlayer(database, gameID, playerID)
	if err != nil {
		return err
	}

	if err := db.GrantShield(database, gameID, playerID, "claim", round); err != nil {
		return err
	}

	notify.SendChannel(s, game.ChannelID, fmt.Sprintf("**%s** claims a shield!", player.DiscordName))
	notify.SendDM(s, playerID, "You have been granted a shield! It will protect you from one murder attempt.")
	return nil
}

// ConsumeShield removes a player's shield (used during night resolution).
func ConsumeShield(database *sql.DB, s *discordgo.Session, gameID int64, playerID string, round int) error {
	if err := db.ConsumeShield(database, gameID, playerID, round); err != nil {
		return err
	}
	notify.SendDM(s, playerID, "Your shield has protected you from murder tonight!")
	return nil
}

// AdminGrantShield grants a shield via admin override.
func AdminGrantShield(database *sql.DB, s *discordgo.Session, gameID int64, playerID string, round int) error {
	return GrantShield(database, s, gameID, playerID, "admin", round)
}
