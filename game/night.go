package game

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/notify"
)

// CastMurderVote records a traitor's murder vote. Returns true if all alive traitors have voted.
func CastMurderVote(database *sql.DB, s *discordgo.Session, gameID int64, round int, voterID, targetID string) (bool, error) {
	game, err := db.GetGameByID(database, gameID)
	if err != nil {
		return false, err
	}

	if game.CurrentPhase != string(PhaseNight) {
		return false, errors.New("it is not night time")
	}

	voter, err := db.GetPlayer(database, gameID, voterID)
	if err != nil || voter.Status != "alive" || voter.Role != string(RoleTraitor) {
		return false, errors.New("only living traitors can vote to murder")
	}

	target, err := db.GetPlayer(database, gameID, targetID)
	if err != nil || target.Status != "alive" {
		return false, errors.New("that player cannot be targeted")
	}

	if target.Role == string(RoleTraitor) {
		return false, errors.New("you cannot murder a fellow traitor")
	}

	if err := db.CastVote(database, gameID, round, "night", voterID, targetID); err != nil {
		return false, err
	}

	if game.TraitorThreadID != "" {
		notify.SendThread(s, game.TraitorThreadID, fmt.Sprintf("**%s** voted to murder **%s**", voter.DiscordName, target.DiscordName))
	}

	// Check if all alive traitors have voted
	voteCount, err := db.CountVotes(database, gameID, round, "night")
	if err != nil {
		return false, nil
	}

	traitors, err := db.GetPlayersByRole(database, gameID, "traitor")
	if err != nil {
		return false, nil
	}

	return voteCount >= len(traitors), nil
}

// ResolveNight tallies murder votes and resolves the night phase.
func ResolveNight(database *sql.DB, s *discordgo.Session, gameID int64, round int) error {
	game, err := db.GetGameByID(database, gameID)
	if err != nil {
		return err
	}

	votes, err := db.GetVotes(database, gameID, round, "night")
	if err != nil {
		return err
	}

	if len(votes) == 0 {
		notify.SendChannel(s, game.ChannelID, "The night passes peacefully. No one was murdered.")
		return nil
	}

	// Count votes per target
	counts := make(map[string]int)
	for _, v := range votes {
		counts[v.TargetDiscordID]++
	}

	// Find max
	maxVotes := 0
	for _, c := range counts {
		if c > maxVotes {
			maxVotes = c
		}
	}

	// On tie, pick alphabetically first
	var topTargets []string
	for id, c := range counts {
		if c == maxVotes {
			topTargets = append(topTargets, id)
		}
	}
	sort.Strings(topTargets)
	targetID := topTargets[0]

	target, err := db.GetPlayer(database, gameID, targetID)
	if err != nil {
		return err
	}

	// Check shield
	if target.HasShield {
		if err := db.ConsumeShield(database, gameID, targetID, round); err != nil {
			slog.Error("consume shield", "error", err)
		}

		notify.SendDM(s, targetID, "Your shield saved you from murder tonight!")
		embed := notify.GameEmbed(
			"Night Phase",
			"The traitors tried to strike, but their target was protected!",
			notify.ColorNight,
			nil,
		)
		embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", round)
		notify.SendEmbed(s, game.ChannelID, embed)
		return nil
	}

	// Murder the target
	if err := db.UpdatePlayerStatus(database, gameID, targetID, string(PlayerMurdered)); err != nil {
		return err
	}

	embed := notify.GameEmbed(
		"Murder!",
		fmt.Sprintf("When morning comes, **%s** was found murdered...", target.DiscordName),
		notify.ColorNight,
		nil,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", round)
	notify.SendEmbed(s, game.ChannelID, embed)

	if err := RevealRole(database, s, gameID, targetID); err != nil {
		slog.Error("reveal role after murder", "error", err)
	}

	return nil
}
