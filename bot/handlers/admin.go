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
	if err := db.AddPlayer(database, gameID, createdBy, i.Member.User.Username); err != nil {
		slog.Error("create game: add creator as player", "error", err, "game_id", gameID)
	}

	// Apply optional buy-in if provided.
	buyinStr := ""
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Name == "buyin" {
			buyinStr = opt.StringValue()
		}
	}
	if buyinStr != "" {
		cents, err := parseDollarsToCents(buyinStr)
		if err != nil || cents <= 0 {
			respondEphemeral(s, i, "Invalid buy-in amount. Use a dollar value like `5` or `10.50`.")
			return
		}
		if err := db.UpdateGameBuyin(database, gameID, cents); err != nil {
			slog.Error("set buyin on create", "error", err)
		}
	}

	description := fmt.Sprintf("A new game of Async Traitors has been created!\n\n**Join Code:** `%s`\n\nPlayers can join with `/join-game code:%s` or click the button below.", code, code)
	if buyinStr != "" {
		cents, _ := parseDollarsToCents(buyinStr)
		description += fmt.Sprintf("\n\n**Buy-in:** %s per player", formatCents(cents))
	}

	embed := notify.GameEmbed(
		"New Game Created!",
		description,
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
		slog.Error("set timers: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if i.Member.User.ID != game.CreatedBy {
		respondEphemeral(s, i, "Only the game creator can change timers.")
		return
	}

	breakfast := game.TimerBreakfastMinutes
	roundtable := game.TimerRoundtableMinutes
	night := game.TimerNightMinutes
	mission := game.TimerMissionMinutes

	opts := i.ApplicationCommandData().Options
	for _, opt := range opts {
		switch opt.Name {
		case "breakfast":
			breakfast = int(opt.IntValue())
		case "roundtable":
			roundtable = int(opt.IntValue())
		case "night":
			night = int(opt.IntValue())
		case "mission":
			mission = int(opt.IntValue())
		}
	}

	if err := db.UpdateGameTimers(database, game.ID, breakfast, roundtable, night, mission); err != nil {
		respondEphemeral(s, i, "Failed to update timers.")
		slog.Error("update timers", "error", err)
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("Timers updated!\nBreakfast: %dm | Round Table: %dm | Night: %dm | Mission: %dm", breakfast, roundtable, night, mission))
}

// HandleSetBuyin sets the buy-in amount for the game (admin, lobby only).
func HandleSetBuyin(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	game, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("set-buyin: game lookup failed", "channel_id", i.ChannelID, "error", err)
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	if i.Member.User.ID != game.CreatedBy {
		respondEphemeral(s, i, "Only the game creator can set the buy-in.")
		return
	}

	if game.Status != "lobby" {
		respondEphemeral(s, i, "Buy-in can only be set while the game is in the lobby.")
		return
	}

	amountStr := i.ApplicationCommandData().Options[0].StringValue()
	cents, err := parseDollarsToCents(amountStr)
	if err != nil || cents <= 0 {
		respondEphemeral(s, i, "Invalid amount. Use a dollar value like `5` or `10.50`.")
		return
	}

	if err := db.UpdateGameBuyin(database, game.ID, cents); err != nil {
		respondEphemeral(s, i, "Failed to set buy-in.")
		slog.Error("set buyin", "error", err)
		return
	}

	embed := notify.GameEmbed(
		"Buy-In Set",
		fmt.Sprintf("The buy-in for this game is **%s** per player. Payments are handled outside Discord.", formatCents(cents)),
		notify.ColorSuccess,
		nil,
	)
	notify.SendEmbed(s, i.ChannelID, embed)

	respondEphemeral(s, i, fmt.Sprintf("Buy-in set to %s.", formatCents(cents)))
}

// parseDollarsToCents converts a dollar string like "5" or "10.50" to cents.
func parseDollarsToCents(s string) (int, error) {
	// Handle formats: "5", "5.0", "5.00", "5.5", "10.50"
	var dollars, cents int
	parts := splitDollarString(s)
	if len(parts) == 1 {
		_, err := fmt.Sscanf(parts[0], "%d", &dollars)
		if err != nil {
			return 0, err
		}
		return dollars * 100, nil
	}
	if len(parts) == 2 {
		_, err := fmt.Sscanf(parts[0], "%d", &dollars)
		if err != nil {
			return 0, err
		}
		centStr := parts[1]
		// Pad or truncate to 2 digits
		if len(centStr) == 1 {
			centStr += "0"
		} else if len(centStr) > 2 {
			centStr = centStr[:2]
		}
		_, err = fmt.Sscanf(centStr, "%d", &cents)
		if err != nil {
			return 0, err
		}
		return dollars*100 + cents, nil
	}
	return 0, fmt.Errorf("invalid format")
}

func splitDollarString(s string) []string {
	idx := -1
	for i, c := range s {
		if c == '.' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}

func formatCents(cents int) string {
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}

// HandleSetHiatus configures quiet hours for the active game.
func HandleSetHiatus(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	game, err := db.GetGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("set hiatus: game lookup failed", "error", err, "channel_id", i.ChannelID)
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
		slog.Error("end game: game lookup failed", "error", err, "channel_id", i.ChannelID)
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

// HandleNukeGames ends all active/lobby games in the guild.
func HandleNukeGames(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	count, err := db.FinishAllGames(database, i.GuildID)
	if err != nil {
		respondEphemeral(s, i, "Failed to nuke games.")
		slog.Error("nuke games", "error", err)
		return
	}

	if count == 0 {
		respondEphemeral(s, i, "No active games to nuke.")
		return
	}

	respondEphemeral(s, i, fmt.Sprintf("Nuked %d game(s). All games in this server are now finished.", count))
}
