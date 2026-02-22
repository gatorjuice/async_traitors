package game

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/notify"
)

// DetermineWinningSide returns "faithful" or "traitors" based on alive traitors at game end.
func DetermineWinningSide(database *sql.DB, gameID int64) string {
	traitors, err := db.GetPlayersByRole(database, gameID, "traitor")
	if err != nil {
		slog.Error("determine winning side: get traitors", "error", err, "game_id", gameID)
	}
	if len(traitors) > 0 {
		return "traitors"
	}
	return "faithful"
}

// FormatCents formats an amount in cents as a dollar string (e.g. 500 → "$5.00").
func FormatCents(cents int) string {
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}

// Payout represents what one player owes or receives.
type Payout struct {
	PlayerDiscordID string
	PlayerName      string
	WalletInfo      string
	Amount          int // cents: per-winner amount owed by losers, or total received by winners
}

// CalculatePayouts determines who owes whom based on game outcome.
// winner is "faithful" or "traitors".
// Returns (winners, losers). Returns empty slices if buyinCents is 0.
func CalculatePayouts(allPlayers []db.Player, winner string, buyinCents int) ([]Payout, []Payout) {
	if buyinCents == 0 {
		return nil, nil
	}

	var winners, losers []Payout

	for _, p := range allPlayers {
		isWinner := false
		if winner == "traitors" {
			isWinner = p.Role == "traitor" && p.Status == "alive"
		} else {
			isWinner = p.Role == "faithful" && p.Status == "alive"
		}

		payout := Payout{
			PlayerDiscordID: p.DiscordID,
			PlayerName:      p.DiscordName,
			WalletInfo:      p.WalletInfo,
		}

		if isWinner {
			winners = append(winners, payout)
		} else {
			losers = append(losers, payout)
		}
	}

	if len(winners) == 0 {
		return winners, losers
	}

	// Each loser owes buyinCents total, split among winners.
	// Each winner receives (numLosers * buyinCents) / numWinners.
	perWinner := (len(losers) * buyinCents) / len(winners)
	for i := range winners {
		winners[i].Amount = perWinner
	}

	// Each loser owes perWinner to each winner (remainder stays with losers).
	perLoserPerWinner := buyinCents / len(winners)
	for i := range losers {
		losers[i].Amount = perLoserPerWinner * len(winners)
	}

	return winners, losers
}

// SendPayoutDMs sends DMs to all losers with a specific winner's wallet info and the amount they owe that winner.
func SendPayoutDMs(s *discordgo.Session, database *sql.DB, gameID int64, winnerDiscordID string) {
	game, err := db.GetGameByID(database, gameID)
	if err != nil {
		slog.Error("send payout DMs: get game", "error", err)
		return
	}

	allPlayers, err := db.GetAllPlayers(database, gameID)
	if err != nil {
		slog.Error("send payout DMs: get players", "error", err)
		return
	}

	winner := DetermineWinningSide(database, gameID)

	winners, losers := CalculatePayouts(allPlayers, winner, game.BuyinAmount)
	if len(winners) == 0 {
		return
	}

	// Find the specific winner who just set their wallet info.
	var winnerPayout Payout
	found := false
	for _, w := range winners {
		if w.PlayerDiscordID == winnerDiscordID {
			winnerPayout = w
			found = true
			break
		}
	}
	if !found {
		return
	}

	// Reload winner's wallet info from DB.
	winnerPlayer, err := db.GetPlayer(database, gameID, winnerDiscordID)
	if err != nil {
		return
	}
	winnerPayout.WalletInfo = winnerPlayer.WalletInfo

	// Each loser owes buyinCents / numWinners to this winner.
	perLoserToThisWinner := game.BuyinAmount / len(winners)

	for _, loser := range losers {
		msg := fmt.Sprintf("**%s** has shared their payment info!\n\nYou owe them **%s**.\n\nPayment info: %s",
			winnerPayout.PlayerName, FormatCents(perLoserToThisWinner), winnerPayout.WalletInfo)
		if err := notify.SendDM(s, loser.PlayerDiscordID, msg); err != nil {
			slog.Error("send payout DM", "error", err, "loser", loser.PlayerDiscordID)
		}
	}
}
