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

// CastBanishmentVote records a public banishment vote.
func CastBanishmentVote(database *sql.DB, s *discordgo.Session, gameID int64, round int, voterID, targetID string) error {
	game, err := db.GetGameByID(database, gameID)
	if err != nil {
		return err
	}

	if game.CurrentPhase != string(PhaseVoting) {
		return errors.New("voting is not open right now")
	}

	voter, err := db.GetPlayer(database, gameID, voterID)
	if err != nil || voter.Status != "alive" {
		return errors.New("you cannot vote")
	}

	target, err := db.GetPlayer(database, gameID, targetID)
	if err != nil || target.Status != "alive" {
		return errors.New("that player cannot be voted for")
	}

	if err := db.CastVote(database, gameID, round, "voting", voterID, targetID); err != nil {
		return err
	}

	notify.SendChannel(s, game.ChannelID, fmt.Sprintf("**%s** voted to banish **%s**", voter.DiscordName, target.DiscordName))
	return nil
}

// TallyBanishmentVotes tallies votes and resolves the banishment.
func TallyBanishmentVotes(database *sql.DB, s *discordgo.Session, gameID int64, round int) (string, error) {
	game, err := db.GetGameByID(database, gameID)
	if err != nil {
		return "", err
	}

	votes, err := db.GetVotes(database, gameID, round, "voting")
	if err != nil {
		return "", err
	}

	if len(votes) == 0 {
		notify.SendChannel(s, game.ChannelID, "No votes were cast. No one is banished.")
		return "", nil
	}

	// Count votes per target
	counts := make(map[string]int)
	for _, v := range votes {
		counts[v.TargetDiscordID]++
	}

	// Find the maximum
	maxVotes := 0
	for _, c := range counts {
		if c > maxVotes {
			maxVotes = c
		}
	}

	// Find all targets with max votes
	var topTargets []string
	for id, c := range counts {
		if c == maxVotes {
			topTargets = append(topTargets, id)
		}
	}

	// Tie = no banishment
	if len(topTargets) > 1 {
		sort.Strings(topTargets)
		notify.SendChannel(s, game.ChannelID, "The vote is tied! The group could not agree. No one is banished.")
		return "", nil
	}

	banishedID := topTargets[0]
	if err := db.UpdatePlayerStatus(database, gameID, banishedID, string(PlayerBanished)); err != nil {
		return "", err
	}

	banished, err := db.GetPlayer(database, gameID, banishedID)
	if err != nil {
		return banishedID, err
	}

	embed := notify.GameEmbed(
		"Banishment!",
		fmt.Sprintf("**%s** has been banished by the group!", banished.DiscordName),
		notify.ColorDanger,
		nil,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", round)
	notify.SendEmbed(s, game.ChannelID, embed)

	if err := RevealRole(database, s, gameID, banishedID); err != nil {
		slog.Error("reveal role after banishment", "error", err)
	}

	return banishedID, nil
}
