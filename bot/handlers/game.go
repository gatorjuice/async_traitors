package handlers

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/notify"
)

// HandleGameInfo displays the current game status.
func HandleGameInfo(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	game, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	playerCount, _ := db.CountPlayersByStatus(database, game.ID, "alive")
	allPlayers, _ := db.GetAllPlayers(database, game.ID)

	fields := []*discordgo.MessageEmbedField{
		{Name: "Join Code", Value: fmt.Sprintf("`%s`", game.JoinCode), Inline: true},
		{Name: "Status", Value: game.Status, Inline: true},
		{Name: "Phase", Value: game.CurrentPhase, Inline: true},
		{Name: "Round", Value: fmt.Sprintf("%d", game.CurrentRound), Inline: true},
		{Name: "Players (alive/total)", Value: fmt.Sprintf("%d/%d", playerCount, len(allPlayers)), Inline: true},
		{Name: "Timers", Value: fmt.Sprintf("Discussion: %dm\nVoting: %dm\nNight: %dm\nCompetition: %dm",
			game.TimerDiscussionMinutes, game.TimerVotingMinutes, game.TimerNightMinutes, game.TimerCompetitionMinutes), Inline: false},
	}

	embed := notify.GameEmbed("Game Info", "", notify.ColorInfo, fields)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Game #%d", game.ID)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

// HandlePlayers lists all players in the game.
func HandlePlayers(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	game, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	players, err := db.GetAllPlayers(database, game.ID)
	if err != nil {
		respondEphemeral(s, i, "Failed to retrieve players.")
		return
	}

	if len(players) == 0 {
		respondEphemeral(s, i, "No players in this game yet.")
		return
	}

	var lines []string
	for idx, p := range players {
		status := "Alive"
		switch p.Status {
		case "banished":
			status = "Banished"
		case "murdered":
			status = "Murdered"
		}
		shield := ""
		if p.HasShield {
			shield = " (shielded)"
		}
		lines = append(lines, fmt.Sprintf("%d. **%s** — %s%s", idx+1, p.DiscordName, status, shield))
	}

	embed := notify.GameEmbed(
		"Players",
		strings.Join(lines, "\n"),
		notify.ColorInfo,
		nil,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

// HandleHelp displays help and game rules.
func HandleHelp(s *discordgo.Session, i *discordgo.InteractionCreate, _ *sql.DB) {
	respondEphemeral(s, i, "Not yet implemented.")
}
