package handlers

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	gamelogic "github.com/gatorjuice/async_traitors/game"
	"github.com/gatorjuice/async_traitors/notify"
)

// HandleJoinGame adds a player to a game by join code.
func HandleJoinGame(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	code := i.ApplicationCommandData().Options[0].StringValue()

	game, err := db.GetGameByJoinCode(database, code)
	if err != nil {
		slog.Error("join game: game lookup failed", "error", err, "join_code", code)
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
		slog.Error("my role: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No active game found in this channel.")
		return
	}

	player, err := db.GetPlayer(database, game.ID, i.Member.User.ID)
	if err != nil {
		slog.Error("my role: player lookup failed", "error", err, "game_id", game.ID, "user_id", i.Member.User.ID)
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
		traitors, err := db.GetPlayersByRole(database, game.ID, "traitor")
		if err != nil {
			slog.Error("my role: get traitors", "error", err, "game_id", game.ID)
		}
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
		slog.Error("join button: game lookup failed", "error", err, "join_code", code)
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

// HandleMarkPaid marks a loser as having paid the calling winner.
func HandleMarkPaid(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	g, err := db.GetFinishedGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("mark paid: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No finished game found in this channel.")
		return
	}

	if g.BuyinAmount == 0 {
		respondEphemeral(s, i, "This game has no buy-in.")
		return
	}

	callerPlayer, err := db.GetPlayer(database, g.ID, i.Member.User.ID)
	if err != nil {
		slog.Error("mark paid: player lookup failed", "error", err, "game_id", g.ID, "user_id", i.Member.User.ID)
		respondEphemeral(s, i, "You are not a player in this game.")
		return
	}

	winnerSide := gamelogic.DetermineWinningSide(database, g.ID)

	isWinner := false
	if winnerSide == "traitors" {
		isWinner = callerPlayer.Role == "traitor" && callerPlayer.Status == "alive"
	} else {
		isWinner = callerPlayer.Role == "faithful" && callerPlayer.Status == "alive"
	}

	if !isWinner {
		respondEphemeral(s, i, "Only winners can mark payments.")
		return
	}

	targetUser := i.ApplicationCommandData().Options[0].UserValue(s)
	targetPlayer, err := db.GetPlayer(database, g.ID, targetUser.ID)
	if err != nil {
		slog.Error("mark paid: target player lookup failed", "error", err, "game_id", g.ID, "target_id", targetUser.ID)
		respondEphemeral(s, i, "That user is not a player in this game.")
		return
	}

	// Verify the target is a loser.
	isLoser := false
	if winnerSide == "traitors" {
		isLoser = targetPlayer.Role != "traitor" || targetPlayer.Status != "alive"
	} else {
		isLoser = targetPlayer.Role != "faithful" || targetPlayer.Status != "alive"
	}

	if !isLoser {
		respondEphemeral(s, i, "That player is a winner, not a loser.")
		return
	}

	if err := db.MarkPaid(database, g.ID, i.Member.User.ID, targetUser.ID); err != nil {
		respondEphemeral(s, i, "Failed to record payment.")
		slog.Error("mark paid", "error", err)
		return
	}

	allPlayers, err := db.GetAllPlayers(database, g.ID)
	if err != nil {
		slog.Error("mark paid: get all players", "error", err, "game_id", g.ID)
	}
	winners, _ := gamelogic.CalculatePayouts(allPlayers, winnerSide, g.BuyinAmount)
	perLoserPerWinner := g.BuyinAmount / len(winners)

	dmMsg := fmt.Sprintf("**%s** has confirmed your payment of **%s**.",
		callerPlayer.DiscordName, gamelogic.FormatCents(perLoserPerWinner))
	if err := notify.SendDM(s, targetUser.ID, dmMsg); err != nil {
		slog.Error("mark-paid DM", "error", err, "loser", targetUser.ID)
	}

	respondEphemeral(s, i, fmt.Sprintf("Marked **%s** as paid.", targetPlayer.DiscordName))
}

// HandlePaymentStatus shows payment status for the calling player.
func HandlePaymentStatus(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB) {
	g, err := db.GetFinishedGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("payment status: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No finished game found in this channel.")
		return
	}

	if g.BuyinAmount == 0 {
		respondEphemeral(s, i, "This game has no buy-in.")
		return
	}

	callerPlayer, err := db.GetPlayer(database, g.ID, i.Member.User.ID)
	if err != nil {
		slog.Error("payment status: player lookup failed", "error", err, "game_id", g.ID, "user_id", i.Member.User.ID)
		respondEphemeral(s, i, "You are not a player in this game.")
		return
	}

	winnerSide := gamelogic.DetermineWinningSide(database, g.ID)
	allPlayers, err := db.GetAllPlayers(database, g.ID)
	if err != nil {
		slog.Error("payment status: get all players", "error", err, "game_id", g.ID)
	}
	winners, losers := gamelogic.CalculatePayouts(allPlayers, winnerSide, g.BuyinAmount)

	isWinner := false
	if winnerSide == "traitors" {
		isWinner = callerPlayer.Role == "traitor" && callerPlayer.Status == "alive"
	} else {
		isWinner = callerPlayer.Role == "faithful" && callerPlayer.Status == "alive"
	}

	if isWinner {
		// Winner view: show each loser and whether they've paid this winner.
		perLoserPerWinner := g.BuyinAmount / len(winners)
		var lines []string
		paidCount := 0
		for _, loser := range losers {
			paid, err := db.IsMarkedPaid(database, g.ID, i.Member.User.ID, loser.PlayerDiscordID)
			if err != nil {
				slog.Error("payment status: check paid", "error", err, "game_id", g.ID, "loser_id", loser.PlayerDiscordID)
			}
			status := "Unpaid"
			if paid {
				status = "Paid"
				paidCount++
			}
			lines = append(lines, fmt.Sprintf("**%s** — %s (%s)", loser.PlayerName, status, gamelogic.FormatCents(perLoserPerWinner)))
		}

		title := fmt.Sprintf("Payment Status (%d/%d paid)", paidCount, len(losers))
		embed := notify.GameEmbed(title, strings.Join(lines, "\n"), notify.ColorInfo, nil)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
	} else {
		// Loser view: show each winner and whether this loser has paid them.
		perLoserPerWinner := g.BuyinAmount / len(winners)
		var lines []string
		paidCount := 0
		for _, winner := range winners {
			paid, err := db.IsMarkedPaid(database, g.ID, winner.PlayerDiscordID, i.Member.User.ID)
			if err != nil {
				slog.Error("payment status: check paid", "error", err, "game_id", g.ID, "winner_id", winner.PlayerDiscordID)
			}
			status := fmt.Sprintf("Owe %s", gamelogic.FormatCents(perLoserPerWinner))
			if paid {
				status = "Paid"
				paidCount++
			}
			lines = append(lines, fmt.Sprintf("**%s** — %s", winner.PlayerName, status))
		}

		title := fmt.Sprintf("Payment Status (%d/%d paid)", paidCount, len(winners))
		embed := notify.GameEmbed(title, strings.Join(lines, "\n"), notify.ColorInfo, nil)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
	}
}

// HandleWallet lets a winner share payment info post-game, triggering DMs to losers.
func HandleWallet(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB, engine *gamelogic.Engine) {
	g, err := db.GetFinishedGameByChannel(database, i.ChannelID)
	if err != nil {
		slog.Error("wallet: game lookup failed", "error", err, "channel_id", i.ChannelID)
		respondEphemeral(s, i, "No finished game found in this channel.")
		return
	}

	if g.BuyinAmount == 0 {
		respondEphemeral(s, i, "This game has no buy-in.")
		return
	}

	player, err := db.GetPlayer(database, g.ID, i.Member.User.ID)
	if err != nil {
		slog.Error("wallet: player lookup failed", "error", err, "game_id", g.ID, "user_id", i.Member.User.ID)
		respondEphemeral(s, i, "You are not a player in this game.")
		return
	}

	winnerSide := gamelogic.DetermineWinningSide(database, g.ID)

	// Check if this player is a winner.
	isWinner := false
	if winnerSide == "traitors" {
		isWinner = player.Role == "traitor" && player.Status == "alive"
	} else {
		isWinner = player.Role == "faithful" && player.Status == "alive"
	}

	if !isWinner {
		respondEphemeral(s, i, "Only winners can share their payment info.")
		return
	}

	walletInfo := i.ApplicationCommandData().Options[0].StringValue()
	if err := db.UpdatePlayerWallet(database, g.ID, i.Member.User.ID, walletInfo); err != nil {
		respondEphemeral(s, i, "Failed to save wallet info.")
		slog.Error("save wallet", "error", err)
		return
	}

	// Send DMs to losers with this winner's payment info.
	gamelogic.SendPayoutDMs(s, database, g.ID, i.Member.User.ID)

	respondEphemeral(s, i, "Your payment info has been shared with all losers!")
}
