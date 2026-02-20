package game

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/notify"
)

// RevealRole announces a player's role in the game channel.
func RevealRole(database *sql.DB, s *discordgo.Session, gameID int64, playerID string) error {
	game, err := db.GetGameByID(database, gameID)
	if err != nil {
		return err
	}

	player, err := db.GetPlayer(database, gameID, playerID)
	if err != nil {
		return err
	}

	alive, err := db.GetAlivePlayers(database, gameID)
	if err != nil {
		return err
	}

	if len(alive) <= game.RevealThreshold {
		notify.SendChannel(s, game.ChannelID, fmt.Sprintf("**%s**'s role will not be revealed (endgame threshold reached).", player.DiscordName))
		return nil
	}

	color := notify.ColorSuccess
	if player.Role == "traitor" {
		color = notify.ColorDanger
	}

	roleName := "FAITHFUL"
	if player.Role == "traitor" {
		roleName = "TRAITOR"
	}

	embed := notify.GameEmbed(
		"Role Revealed",
		fmt.Sprintf("**%s** was a **%s**!", player.DiscordName, roleName),
		color,
		nil,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)

	_, err = notify.SendEmbed(s, game.ChannelID, embed)
	if err != nil {
		slog.Error("send reveal embed", "error", err)
	}
	return nil
}
