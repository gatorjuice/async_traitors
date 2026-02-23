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

// HandleGameInfo displays the current game status.
func HandleGameInfo(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	game, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("game info: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	playerCount, err := db.CountPlayersByStatus(database, game.ID, "alive")
	if err != nil {
		slog.Error("game info: count alive players", "error", err, "game_id", game.ID)
	}
	allPlayers, err := db.GetAllPlayers(database, game.ID)
	if err != nil {
		slog.Error("game info: get all players", "error", err, "game_id", game.ID)
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "Join Code", Value: fmt.Sprintf("`%s`", game.JoinCode), Inline: true},
		{Name: "Status", Value: game.Status, Inline: true},
		{Name: "Phase", Value: game.CurrentPhase, Inline: true},
		{Name: "Round", Value: fmt.Sprintf("%d", game.CurrentRound), Inline: true},
		{Name: "Players (alive/total)", Value: fmt.Sprintf("%d/%d", playerCount, len(allPlayers)), Inline: true},
		{Name: "Timers", Value: fmt.Sprintf("Breakfast: %dm\nRound Table: %dm\nNight: %dm\nMission: %dm",
			game.TimerBreakfastMinutes, game.TimerRoundtableMinutes, game.TimerNightMinutes, game.TimerMissionMinutes), Inline: false},
	}

	if game.HiatusStart != "" && game.HiatusEnd != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "Quiet Hours",
			Value: fmt.Sprintf("%s–%s (%s)", game.HiatusStart, game.HiatusEnd, game.HiatusTimezone),
		})
	}

	if game.EndBy != "" {
		if deadline, err := parseDeadline(game.EndBy, game.HiatusTimezone); err == nil {
			tzLabel := "UTC"
			if game.HiatusTimezone != "" {
				tzLabel = game.HiatusTimezone
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  "Deadline",
				Value: fmt.Sprintf("<t:%d:F> (%s)", deadline.Unix(), tzLabel),
			})
		}
	}

	if game.BuyinAmount > 0 {
		potTotal := len(allPlayers) * game.BuyinAmount
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Buy-In",
			Value:  fmt.Sprintf("$%d.%02d per player", game.BuyinAmount/100, game.BuyinAmount%100),
			Inline: true,
		})
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Prize Pool",
			Value:  fmt.Sprintf("$%d.%02d", potTotal/100, potTotal%100),
			Inline: true,
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
		slog.Error("players: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	players, err := db.GetAllPlayers(database, game.ID)
	if err != nil {
		slog.Error("players: get all players", "error", err, "game_id", game.ID)
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
			Value: "**Breakfast** — Discover who was murdered overnight\n**Mission** — Compete for shields\n**Round Table** — Vote to banish a suspect\n**Night** — Traitors choose a victim (or recruit)",
		},
		{
			Name:  "Player Commands",
			Value: "`/join-game` — Join with a code\n`/my-role` — Check your role (DM)\n`/vote` — Vote to banish\n`/murder-vote` — Traitors vote to murder\n`/recruit` — Traitors recruit (recruitment night)\n`/accept-recruitment` — Accept traitor offer\n`/refuse-recruitment` — Refuse traitor offer\n`/submit-answer` — Answer mission\n`/claim-shield` — Claim a shield\n`/game-info` — Game status\n`/players` — Player list\n`/wallet` — Share payment info (winners, post-game)\n`/mark-paid` — Mark a loser as paid (winners, post-game)\n`/payment-status` — View payment status (post-game)",
		},
		{
			Name:  "Admin Commands",
			Value: "`/create-game` — Create a game\n`/start-game` — Start the game\n`/start-mission` — Begin mission\n`/end-mission` — End mission\n`/grant-shield` — Give a shield\n`/force-recruit` — Force recruit a player\n`/set-timers` — Set phase timers\n`/set-buyin` — Set buy-in amount (lobby)\n`/advance-phase` — Skip to next phase\n`/end-game` — Force end",
		},
		{
			Name:  "Shields",
			Value: "Shields protect you from one murder attempt. Earn them by winning missions or claiming them during scavenger hunts. The shield is consumed when it blocks a murder.",
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

	shieldLog, err := db.GetShieldLog(database, game.ID)
	if err != nil {
		slog.Error("recap: get shield log", "error", err, "game_id", game.ID)
	}

	var timeline string
	for round := 1; round <= game.CurrentRound; round++ {
		timeline += fmt.Sprintf("**Round %d**\n", round)

		banished, err := db.GetPlayersByStatusAndRound(database, game.ID, "banished", round)
		if err != nil {
			slog.Error("recap: get banished", "error", err, "game_id", game.ID, "round", round)
		}
		murdered, err := db.GetPlayersByStatusAndRound(database, game.ID, "murdered", round)
		if err != nil {
			slog.Error("recap: get murdered", "error", err, "game_id", game.ID, "round", round)
		}

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

	alive, err := db.GetAlivePlayers(database, game.ID)
	if err != nil {
		slog.Error("recap: get alive players", "error", err, "game_id", game.ID)
	}
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

// HandleRules displays the full detailed rules of The Traitors.
func HandleRules(s *discordgo.Session, i *discordgo.InteractionCreate, _ *sql.DB) {
	embed1 := notify.GameEmbed(
		"The Traitors — Full Rules",
		"A social deduction game inspired by the TV show. Players are secretly divided into **Faithful** and **Traitors**. "+
			"The Faithful must identify and banish all Traitors. The Traitors must survive and eliminate the Faithful until they hold the majority.",
		notify.ColorInfo,
		[]*discordgo.MessageEmbedField{
			{
				Name: "Setup",
				Value: "When the game starts, roles are assigned secretly via DM.\n\n" +
					"**Traitors** — Roughly 1 per 4 players (minimum 1). They know each other and share a private thread.\n" +
					"**Faithful** — Everyone else. They do not know who the Traitors are.\n\n" +
					"The game plays out in rounds. Each round has four phases.",
			},
			{
				Name: "Phase 1: Breakfast",
				Value: "Players gather and discover who was **murdered** overnight by the Traitors. " +
					"The victim's role may be revealed (unless the endgame threshold has been reached). " +
					"This is a time to discuss, react, and begin forming suspicions.\n\n" +
					"On Round 1, no one has been murdered yet — Breakfast is simply the opening gathering.",
			},
			{
				Name: "Phase 2: Mission",
				Value: "A group challenge is presented (trivia, speed math, word puzzle, or scavenger hunt). " +
					"The winner earns a **Shield** — a one-time protection from murder.\n\n" +
					"The admin starts and ends each mission with `/start-mission` and `/end-mission`. " +
					"Players submit answers with `/submit-answer`.",
			},
			{
				Name: "Phase 3: Round Table",
				Value: "All alive players vote to **banish** one person from the game using `/vote`.\n\n" +
					"• Votes are **secret** until everyone has voted or the timer expires.\n" +
					"• The player with the most votes is banished and their role is revealed.\n" +
					"• If there is a **tie**, no one is banished.\n" +
					"• The phase auto-advances once all alive players have voted.",
			},
		},
	)
	embed1.Footer.Text = "Async Traitors | Rules (1/2)"

	embed2 := notify.GameEmbed(
		"",
		"",
		notify.ColorInfo,
		[]*discordgo.MessageEmbedField{
			{
				Name: "Phase 4: Night",
				Value: "The Traitors secretly choose a victim to **murder** using `/murder-vote` in their private thread.\n\n" +
					"• Traitors vote among themselves — majority wins, ties broken alphabetically.\n" +
					"• If the target has a **Shield**, the murder is blocked and the shield is consumed.\n" +
					"• The murder is not announced until the next Breakfast.\n" +
					"• The phase auto-advances once all Traitors have voted.",
			},
			{
				Name: "Recruitment",
				Value: "If a Traitor is banished and fewer than **2 Traitors remain** (and the game is not at endgame), " +
					"the next Night becomes a **Recruitment Night** instead of a murder night.\n\n" +
					"• The remaining Traitors choose a Faithful player to recruit using `/recruit`.\n" +
					"• The chosen player receives a DM ultimatum:\n" +
					"  — `/accept-recruitment` → Become a Traitor (added to the traitor thread).\n" +
					"  — `/refuse-recruitment` → You are murdered instead.\n" +
					"• Traitors recruit **or** murder — never both in the same night.\n" +
					"• The admin can also force-recruit with `/force-recruit`.",
			},
			{
				Name: "Shields",
				Value: "• Earned by winning missions or claimed during scavenger hunts (`/claim-shield`).\n" +
					"• Admins can grant shields with `/grant-shield`.\n" +
					"• A shield protects you from **one murder attempt** — it is consumed when used.\n" +
					"• Shields do **not** protect against banishment.",
			},
			{
				Name: "Win Conditions",
				Value: "**Faithful win** — All Traitors have been banished.\n" +
					"**Traitors win** — The number of Traitors equals or exceeds the number of Faithful.\n\n" +
					"When the game ends, all remaining players' roles are revealed.",
			},
			{
				Name: "Role Reveal & Endgame Threshold",
				Value: "When a player is banished or murdered, their role is normally revealed to the group. " +
					"However, once the number of alive players drops to the **endgame threshold** (default: 4), " +
					"roles are no longer revealed — keeping the final rounds tense and uncertain.",
			},
		},
	)
	embed2.Footer.Text = "Async Traitors | Rules (2/2)"

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed1, embed2},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}
