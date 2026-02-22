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

	// Progress indicator in traitor thread
	if game.TraitorThreadID != "" {
		notify.SendThread(s, game.TraitorThreadID,
			fmt.Sprintf("(%d of %d traitors have voted)", voteCount, len(traitors)))
	}

	return voteCount >= len(traitors), nil
}

// ResolveNight tallies murder votes and resolves the night phase silently.
// The murder result is NOT announced publicly — it will be revealed at Breakfast.
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
		// No votes cast — notify traitor thread only
		if game.TraitorThreadID != "" {
			notify.SendThread(s, game.TraitorThreadID, "No murder votes were cast tonight.")
		}
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

		// DM the shielded player privately
		notify.SendDM(s, targetID, "Your shield saved you from murder tonight!")

		// Notify traitor thread
		if game.TraitorThreadID != "" {
			notify.SendThread(s, game.TraitorThreadID,
				fmt.Sprintf("Your target **%s** was protected by a shield! No one was murdered.", target.DiscordName))
		}
		return nil
	}

	// Murder the target silently (no public announcement — revealed at Breakfast)
	if err := db.UpdatePlayerStatusWithRound(database, gameID, targetID, string(PlayerMurdered), round); err != nil {
		return err
	}

	// Notify traitor thread of the result
	if game.TraitorThreadID != "" {
		notify.SendThread(s, game.TraitorThreadID,
			fmt.Sprintf("**%s** has been murdered. The group will discover this at Breakfast.", target.DiscordName))
	}

	return nil
}
