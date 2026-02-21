package handlers

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log/slog"
	"math/big"
	"regexp"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/notify"
)

var hhmmPattern = regexp.MustCompile(`^\d{2}:\d{2}$`)

const joinCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateJoinCode() (string, error) {
	code := make([]byte, 6)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(joinCodeChars))))
		if err != nil {
			return "", err
		}
		code[i] = joinCodeChars[n.Int64()]
	}
	return string(code), nil
}

// HandleCreateGame creates a new game in the current channel.
func HandleCreateGame(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	code, err := generateJoinCode()
	if err != nil {
		respondEphemeral(s, i, "Failed to generate join code.")
		slog.Error("generate join code", "error", err)
		return
	}

	channelID := i.ChannelID
	guildID := i.GuildID
	createdBy := i.Member.User.ID

	gameID, err := db.CreateGame(database, code, guildID, channelID, createdBy)
	if err != nil {
		respondEphemeral(s, i, "Failed to create game. Is there already an active game in this channel?")
		slog.Error("create game", "error", err)
		return
	}

	// Add creator as first player
	_ = db.AddPlayer(database, gameID, createdBy, i.Member.User.Username)

	embed := notify.GameEmbed(
		"New Game Created!",
		fmt.Sprintf("A new game of Async Traitors has been created!\n\n**Join Code:** `%s`\n\nPlayers can join with `/join-game code:%s` or click the button below.", code, code),
		notify.ColorSuccess,
		nil,
	)
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "Join Game",
					Style:    discordgo.SuccessButton,
					CustomID: "join-game:" + code,
				},
			},
		},
	}
	notify.SendEmbedWithComponents(s, channelID, embed, components)

	respondEphemeral(s, i, fmt.Sprintf("Game created! Join code: **%s** (Game #%d)", code, gameID))
}

// HandleSetTimers updates the timer settings for the active game.
func HandleSetTimers(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	game, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if i.Member.User.ID != game.CreatedBy {
		respondEphemeral(s, i, "Only the game creator can change timers.")
		return
	}

	discussion := game.TimerDiscussionMinutes
	voting := game.TimerVotingMinutes
	night := game.TimerNightMinutes
	competition := game.TimerCompetitionMinutes

	opts := i.ApplicationCommandData().Options
	for _, opt := range opts {
		switch opt.Name {
		case "discussion":
			discussion = int(opt.IntValue())
		case "voting":
			voting = int(opt.IntValue())
		case "night":
			night = int(opt.IntValue())
		case "competition":
			competition = int(opt.IntValue())
		}
	}

	if err := db.UpdateGameTimers(database, game.ID, discussion, voting, night, competition); err != nil {
		respondEphemeral(s, i, "Failed to update timers.")
		slog.Error("update timers", "error", err)
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("Timers updated!\nDiscussion: %dm | Voting: %dm | Night: %dm | Competition: %dm", discussion, voting, night, competition))
}

// HandleSetHiatus configures quiet hours for the active game.
func HandleSetHiatus(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	game, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if i.Member.User.ID != game.CreatedBy {
		respondEphemeral(s, i, "Only the game creator can set quiet hours.")
		return
	}

	opts := i.ApplicationCommandData().Options
	start := opts[0].StringValue()
	end := opts[1].StringValue()
	tz := opts[2].StringValue()

	if !hhmmPattern.MatchString(start) || !hhmmPattern.MatchString(end) {
		respondEphemeral(s, i, "Invalid time format. Use HH:MM (e.g. 22:00).")
		return
	}

	if _, err := time.LoadLocation(tz); err != nil {
		respondEphemeral(s, i, "Invalid timezone. Use IANA format (e.g. America/New_York, Europe/London, UTC).")
		return
	}

	if err := db.UpdateGameHiatus(database, game.ID, start, end, tz); err != nil {
		respondEphemeral(s, i, "Failed to update quiet hours.")
		slog.Error("update hiatus", "error", err)
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("Quiet hours set: %s–%s (%s). Timers will pause during this window.", start, end, tz))
}

// HandleEndGame force-ends the active game.
func HandleEndGame(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	game, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if i.Member.User.ID != game.CreatedBy {
		respondEphemeral(s, i, "Only the game creator can end the game.")
		return
	}

	if err := db.UpdateGameStatus(database, game.ID, "finished", "finished"); err != nil {
		respondEphemeral(s, i, "Failed to end game.")
		slog.Error("end game", "error", err)
		return
	}

	embed := notify.GameEmbed(
		"Game Over",
		"The game has been ended by the host.",
		notify.ColorDanger,
		nil,
	)
	notify.SendEmbed(s, i.ChannelID, embed)

	respondEphemeral(s, i, "Game ended.")
}
