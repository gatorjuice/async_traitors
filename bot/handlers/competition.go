package handlers

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/competition"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/game"
	"github.com/gatorjuice/async_traitors/notify"
)

// HandleStartCompetition starts a competition round.
func HandleStartCompetition(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if i.Member.User.ID != g.CreatedBy {
		respondEphemeral(s, i, "Only the game creator can start competitions.")
		return
	}

	if g.CurrentPhase != "competition" {
		respondEphemeral(s, i, "It's not competition phase right now.")
		return
	}

	compType := i.ApplicationCommandData().Options[0].StringValue()
	comp, ok := competition.Get(compType)
	if !ok {
		respondEphemeral(s, i, "Unknown competition type: "+compType)
		return
	}

	question, answer, data, err := comp.Generate()
	if err != nil {
		respondEphemeral(s, i, "Failed to generate competition.")
		slog.Error("generate competition", "error", err)
		return
	}

	_, err = db.CreateCompetition(database, g.ID, g.CurrentRound, compType, data, answer)
	if err != nil {
		respondEphemeral(s, i, "Failed to create competition.")
		slog.Error("create competition", "error", err)
		return
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "Type", Value: compType, Inline: true},
	}

	if compType == "scavenger" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "How to Complete",
			Value: "Use `/claim-shield` when you've completed the challenge!",
		})
	} else {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  "How to Answer",
			Value: "Use `/submit-answer answer:your_answer` to submit!",
		})
	}

	embed := notify.GameEmbed(
		"Competition: "+comp.Description(),
		question,
		notify.ColorInfo,
		fields,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", g.CurrentRound)
	notify.SendEmbed(s, g.ChannelID, embed)

	respondEphemeral(s, i, "Competition started!")
}

// HandleSubmitAnswer submits a competition answer.
func HandleSubmitAnswer(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	g, _ := requirePlayer(s, i, database)
	if g == nil {
		return
	}

	activeComp, err := db.GetActiveCompetition(database, g.ID)
	if err != nil {
		respondEphemeral(s, i, "No active competition right now.")
		return
	}

	comp, ok := competition.Get(activeComp.CompType)
	if !ok {
		respondEphemeral(s, i, "Unknown competition type.")
		return
	}

	answer := i.ApplicationCommandData().Options[0].StringValue()
	correct := comp.CheckAnswer(answer, activeComp.Answer)

	elapsed := time.Since(activeComp.CreatedAt).Milliseconds()
	if err := db.SubmitCompetitionResult(database, activeComp.ID, i.Member.User.ID, answer, correct, elapsed); err != nil {
		respondEphemeral(s, i, "Failed to submit answer.")
		slog.Error("submit answer", "error", err)
		return
	}

	// Announce submission
	notify.SendChannel(s, g.ChannelID,
		fmt.Sprintf("**%s** has submitted an answer!", i.Member.User.Username))

	if correct {
		respondEphemeral(s, i, "Correct!")

		// For non-speed types, first correct answer wins
		if activeComp.CompType != "speed" && activeComp.CompType != "scavenger" {
			results, _ := db.GetCompetitionResults(database, activeComp.ID)
			correctCount := 0
			for _, r := range results {
				if r.Correct {
					correctCount++
				}
			}
			if correctCount == 1 {
				// This was the first correct answer — award shield
				game.GrantShield(database, s, g.ID, i.Member.User.ID, "competition", g.CurrentRound)
				notify.SendChannel(s, g.ChannelID, fmt.Sprintf("**%s** answered correctly first and earned a shield!", i.Member.User.Username))
			}
		}
	} else {
		respondEphemeral(s, i, "Incorrect, try again!")
	}
}

// HandleEndCompetition ends the current competition.
func HandleEndCompetition(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	g, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if i.Member.User.ID != g.CreatedBy {
		respondEphemeral(s, i, "Only the game creator can end competitions.")
		return
	}

	activeComp, err := db.GetActiveCompetition(database, g.ID)
	if err != nil {
		respondEphemeral(s, i, "No active competition to end.")
		return
	}

	results, _ := db.GetCompetitionResults(database, activeComp.ID)

	// For speed competitions, award shield to fastest correct answer
	if activeComp.CompType == "speed" {
		var winnerID string
		var bestTime int64 = -1
		for _, r := range results {
			if r.Correct && (bestTime < 0 || r.TimeMs < bestTime) {
				bestTime = r.TimeMs
				winnerID = r.PlayerDiscordID
			}
		}
		if winnerID != "" {
			game.GrantShield(database, s, g.ID, winnerID, "competition", g.CurrentRound)
			notify.SendChannel(s, g.ChannelID, fmt.Sprintf("<@%s> was the fastest and earned a shield!", winnerID))
		}
	}

	if err := db.EndCompetition(database, activeComp.ID); err != nil {
		respondEphemeral(s, i, "Failed to end competition.")
		slog.Error("end competition", "error", err)
		return
	}

	// Build leaderboard
	alive, _ := db.GetAlivePlayers(database, g.ID)
	playerNames := make(map[string]string)
	for _, p := range alive {
		playerNames[p.DiscordID] = p.DiscordName
	}

	var leaderboard string
	correctCount := 0
	rank := 1
	for _, r := range results {
		if !r.Correct {
			continue
		}
		correctCount++
		name := r.PlayerDiscordID
		if n, ok := playerNames[r.PlayerDiscordID]; ok {
			name = n
		}
		if activeComp.CompType == "speed" {
			leaderboard += fmt.Sprintf("%d. **%s** — %.1fs\n", rank, name, float64(r.TimeMs)/1000)
		} else {
			leaderboard += fmt.Sprintf("%d. **%s**\n", rank, name)
		}
		rank++
	}

	if leaderboard == "" {
		leaderboard = "No correct answers!"
	}

	summary := fmt.Sprintf("%d of %d players participated.\n\n**Leaderboard:**\n%s",
		len(results), len(alive), leaderboard)

	embed := notify.GameEmbed("Competition Results", summary, notify.ColorSuccess, nil)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", g.CurrentRound)
	notify.SendEmbed(s, g.ChannelID, embed)

	respondEphemeral(s, i, "Competition ended!")
}
