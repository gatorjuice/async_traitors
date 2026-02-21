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

	if game.HiatusStart != "" && game.HiatusEnd != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "Quiet Hours",
			Value: fmt.Sprintf("%s–%s (%s)", game.HiatusStart, game.HiatusEnd, game.HiatusTimezone),
		})
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
	fields := []*discordgo.MessageEmbedField{
		{
			Name:  "How to Play",
			Value: "A social deduction game played over a long weekend. Some players are secretly Traitors trying to eliminate the Faithful. The Faithful must identify and banish the Traitors before it's too late.",
		},
		{
			Name:  "Roles",
			Value: "**Traitor** — Secretly murder faithful players at night. Win when traitors >= faithful.\n**Faithful** — Find and banish all traitors through voting. Win when all traitors are gone.",
		},
		{
			Name:  "Game Phases",
			Value: "**Competition** — Compete for shields\n**Discussion** — Talk and strategize\n**Voting** — Vote to banish a suspect\n**Night** — Traitors choose a victim",
		},
		{
			Name:  "Player Commands",
			Value: "`/join-game` — Join with a code\n`/my-role` — Check your role (DM)\n`/vote` — Vote to banish\n`/murder-vote` — Traitors vote to murder\n`/submit-answer` — Answer competition\n`/claim-shield` — Claim a shield\n`/game-info` — Game status\n`/players` — Player list",
		},
		{
			Name:  "Admin Commands",
			Value: "`/create-game` — Create a game\n`/start-game` — Start the game\n`/start-competition` — Begin competition\n`/end-competition` — End competition\n`/grant-shield` — Give a shield\n`/set-timers` — Set phase timers\n`/advance-phase` — Skip to next phase\n`/end-game` — Force end",
		},
		{
			Name:  "Shields",
			Value: "Shields protect you from one murder attempt. Earn them by winning competitions or claiming them during scavenger hunts. The shield is consumed when it blocks a murder.",
		},
		{
			Name:  "Win Conditions",
			Value: "**Faithful win** — All traitors are banished\n**Traitors win** — Traitors outnumber or equal the faithful",
		},
	}

	embed := notify.GameEmbed("Async Traitors — Help", "A social deduction game for Discord", notify.ColorInfo, fields)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

// HandleRecap shows the game timeline for all rounds.
func HandleRecap(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	game, _ := requirePlayer(s, i, database)
	if game == nil {
		return
	}

	if game.CurrentRound == 0 {
		respondEphemeral(s, i, "The game hasn't started yet.")
		return
	}

	shieldLog, _ := db.GetShieldLog(database, game.ID)

	var timeline string
	for round := 1; round <= game.CurrentRound; round++ {
		timeline += fmt.Sprintf("**Round %d**\n", round)

		banished, _ := db.GetPlayersByStatusAndRound(database, game.ID, "banished", round)
		murdered, _ := db.GetPlayersByStatusAndRound(database, game.ID, "murdered", round)

		if len(banished) > 0 {
			for _, p := range banished {
				roleName := "Faithful"
				if p.Role == "traitor" {
					roleName = "Traitor"
				}
				timeline += fmt.Sprintf("  Banished: **%s** (%s)\n", p.DiscordName, roleName)
			}
		} else {
			timeline += "  No banishment\n"
		}

		if len(murdered) > 0 {
			for _, p := range murdered {
				roleName := "Faithful"
				if p.Role == "traitor" {
					roleName = "Traitor"
				}
				timeline += fmt.Sprintf("  Murdered: **%s** (%s)\n", p.DiscordName, roleName)
			}
		} else {
			// Check for shield blocks this round
			shieldUsed := false
			for _, entry := range shieldLog {
				if entry.RoundUsed != nil && *entry.RoundUsed == round {
					shieldUsed = true
					break
				}
			}
			if shieldUsed {
				timeline += "  Shield blocked the attack\n"
			} else {
				timeline += "  Peaceful night\n"
			}
		}

		timeline += "\n"
	}

	alive, _ := db.GetAlivePlayers(database, game.ID)
	timeline += fmt.Sprintf("**%d players remaining**", len(alive))

	embed := notify.GameEmbed("Game Recap", timeline, notify.ColorInfo, nil)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Game #%d", game.ID)

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}
