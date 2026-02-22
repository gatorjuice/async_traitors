package game

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/notify"
)

// RecruitPlayer records a traitor's recruitment choice. Returns true if all alive traitors have voted.
func RecruitPlayer(database *sql.DB, s *discordgo.Session, gameID int64, round int, traitorID, targetID string) (bool, error) {
	game, err := db.GetGameByID(database, gameID)
	if err != nil {
		return false, err
	}

	if game.CurrentPhase != string(PhaseNight) {
		return false, errors.New("it is not night time")
	}

	if !game.RecruitmentPending {
		return false, errors.New("this is not a recruitment night")
	}

	voter, err := db.GetPlayer(database, gameID, traitorID)
	if err != nil || voter.Status != "alive" || voter.Role != string(RoleTraitor) {
		return false, errors.New("only living traitors can recruit")
	}

	target, err := db.GetPlayer(database, gameID, targetID)
	if err != nil || target.Status != "alive" {
		return false, errors.New("that player cannot be recruited")
	}

	if target.Role == string(RoleTraitor) {
		return false, errors.New("you cannot recruit a fellow traitor")
	}

	// Use "recruit" phase in votes table
	if err := db.CastVote(database, gameID, round, "recruit", traitorID, targetID); err != nil {
		return false, err
	}

	if game.TraitorThreadID != "" {
		notify.SendThread(s, game.TraitorThreadID,
			fmt.Sprintf("**%s** voted to recruit **%s**", voter.DiscordName, target.DiscordName))
	}

	// Check if all alive traitors have voted
	voteCount, err := db.CountVotes(database, gameID, round, "recruit")
	if err != nil {
		return false, nil
	}

	traitors, err := db.GetPlayersByRole(database, gameID, "traitor")
	if err != nil {
		return false, nil
	}

	if game.TraitorThreadID != "" {
		notify.SendThread(s, game.TraitorThreadID,
			fmt.Sprintf("(%d of %d traitors have voted)", voteCount, len(traitors)))
	}

	return voteCount >= len(traitors), nil
}

// ResolveRecruitment determines the recruitment target and sends the ultimatum.
func ResolveRecruitment(database *sql.DB, s *discordgo.Session, gameID int64, round int) error {
	game, err := db.GetGameByID(database, gameID)
	if err != nil {
		return err
	}

	votes, err := db.GetVotes(database, gameID, round, "recruit")
	if err != nil {
		return err
	}

	if len(votes) == 0 {
		// No recruitment votes — auto-refuse (timer expired)
		if err := db.SetRecruitmentPending(database, gameID, false); err != nil {
			slog.Error("clear recruitment pending", "error", err)
		}
		if game.TraitorThreadID != "" {
			notify.SendThread(s, game.TraitorThreadID, "No recruitment votes were cast. The opportunity has passed.")
		}
		return nil
	}

	// Tally votes — majority wins, tie = first alphabetical
	counts := make(map[string]int)
	for _, v := range votes {
		counts[v.TargetDiscordID]++
	}

	maxVotes := 0
	for _, c := range counts {
		if c > maxVotes {
			maxVotes = c
		}
	}

	var topTargets []string
	for id, c := range counts {
		if c == maxVotes {
			topTargets = append(topTargets, id)
		}
	}

	// Pick first alphabetically on tie
	if len(topTargets) > 1 {
		sortStrings(topTargets)
	}
	targetID := topTargets[0]

	target, err := db.GetPlayer(database, gameID, targetID)
	if err != nil {
		return err
	}

	// Send ultimatum DM
	notify.SendDM(s, targetID,
		fmt.Sprintf("The traitors have chosen you for recruitment. You must decide:\n\n"+
			"Use `/accept-recruitment` to join the traitors.\n"+
			"Use `/refuse-recruitment` to refuse (you will be murdered).\n\n"+
			"Choose wisely, **%s**.", target.DiscordName))

	// Notify traitor thread
	if game.TraitorThreadID != "" {
		notify.SendThread(s, game.TraitorThreadID,
			fmt.Sprintf("The ultimatum has been sent to **%s**. Waiting for their response...", target.DiscordName))
	}

	return nil
}

// AcceptRecruitment handles a player accepting the traitor recruitment offer.
func AcceptRecruitment(e *Engine, gameID int64, playerID string) error {
	game, err := db.GetGameByID(e.DB, gameID)
	if err != nil {
		return err
	}

	if !game.RecruitmentPending {
		return errors.New("no recruitment is pending")
	}

	player, err := db.GetPlayer(e.DB, gameID, playerID)
	if err != nil {
		return err
	}

	if player.Role != string(RoleFaithful) || player.Status != "alive" {
		return errors.New("you cannot be recruited")
	}

	// Verify this player was the recruitment target
	votes, err := db.GetVotes(e.DB, gameID, game.CurrentRound, "recruit")
	if err != nil {
		return err
	}
	counts := make(map[string]int)
	for _, v := range votes {
		counts[v.TargetDiscordID]++
	}
	maxVotes := 0
	maxTarget := ""
	for id, c := range counts {
		if c > maxVotes {
			maxVotes = c
			maxTarget = id
		}
	}
	if maxTarget != playerID {
		return errors.New("you were not selected for recruitment")
	}

	// Change role to traitor
	if err := db.UpdatePlayerRole(e.DB, gameID, playerID, string(RoleTraitor)); err != nil {
		return err
	}

	// Record recruited round
	if err := db.SetPlayerRecruitedRound(e.DB, gameID, playerID, game.CurrentRound); err != nil {
		slog.Error("set recruited round", "error", err)
	}

	// Add to traitor thread
	if game.TraitorThreadID != "" {
		notify.AddToThread(e.Session, game.TraitorThreadID, playerID)
		notify.SendThread(e.Session, game.TraitorThreadID,
			fmt.Sprintf("Welcome, **%s**! You are now a traitor.", player.DiscordName))
	}

	// DM the new traitor
	traitors, err := db.GetPlayersByRole(e.DB, gameID, "traitor")
	if err != nil {
		slog.Error("accept recruitment: get traitors", "error", err, "game_id", gameID)
	}
	var others []string
	for _, t := range traitors {
		if t.DiscordID != playerID {
			others = append(others, t.DiscordName)
		}
	}
	msg := "You have accepted. You are now a **TRAITOR**!"
	if len(others) > 0 {
		msg += fmt.Sprintf("\nYour fellow traitors: %s", joinStrings(others))
	}
	notify.SendDM(e.Session, playerID, msg)

	// Public announcement (vague)
	notify.SendChannel(e.Session, game.ChannelID, "The night passes. No one was murdered... but something has changed.")

	// Clear recruitment pending
	if err := db.SetRecruitmentPending(e.DB, gameID, false); err != nil {
		slog.Error("clear recruitment pending", "error", err)
	}

	// Advance phase
	return e.AdvancePhase(gameID)
}

// RefuseRecruitment handles a player refusing the traitor recruitment offer.
func RefuseRecruitment(e *Engine, gameID int64, playerID string) error {
	game, err := db.GetGameByID(e.DB, gameID)
	if err != nil {
		return err
	}

	if !game.RecruitmentPending {
		return errors.New("no recruitment is pending")
	}

	player, err := db.GetPlayer(e.DB, gameID, playerID)
	if err != nil {
		return err
	}

	if player.Status != "alive" {
		return errors.New("you are not alive")
	}

	// Verify this player was the recruitment target
	votes, err := db.GetVotes(e.DB, gameID, game.CurrentRound, "recruit")
	if err != nil {
		return err
	}
	counts := make(map[string]int)
	for _, v := range votes {
		counts[v.TargetDiscordID]++
	}
	maxVotes := 0
	maxTarget := ""
	for id, c := range counts {
		if c > maxVotes {
			maxVotes = c
			maxTarget = id
		}
	}
	if maxTarget != playerID {
		return errors.New("you were not selected for recruitment")
	}

	// Murder the player for refusing
	if err := db.UpdatePlayerStatusWithRound(e.DB, gameID, playerID, string(PlayerMurdered), game.CurrentRound); err != nil {
		return err
	}

	// Notify traitor thread
	if game.TraitorThreadID != "" {
		notify.SendThread(e.Session, game.TraitorThreadID,
			fmt.Sprintf("**%s** refused recruitment. They have been eliminated.", player.DiscordName))
	}

	// DM the refused player
	notify.SendDM(e.Session, playerID, "You refused the traitors' offer. You have been murdered.")

	// Clear recruitment pending
	if err := db.SetRecruitmentPending(e.DB, gameID, false); err != nil {
		slog.Error("clear recruitment pending", "error", err)
	}

	// Advance phase (murder will be revealed at next Breakfast)
	return e.AdvancePhase(gameID)
}

// ForceRecruit directly recruits a player as a traitor (admin command).
func ForceRecruit(e *Engine, gameID int64, playerID string) error {
	game, err := db.GetGameByID(e.DB, gameID)
	if err != nil {
		return err
	}

	player, err := db.GetPlayer(e.DB, gameID, playerID)
	if err != nil {
		return err
	}

	if player.Status != "alive" {
		return errors.New("that player is not alive")
	}

	if player.Role == string(RoleTraitor) {
		return errors.New("that player is already a traitor")
	}

	// Change role
	if err := db.UpdatePlayerRole(e.DB, gameID, playerID, string(RoleTraitor)); err != nil {
		return err
	}

	// Record recruited round
	if err := db.SetPlayerRecruitedRound(e.DB, gameID, playerID, game.CurrentRound); err != nil {
		slog.Error("set recruited round", "error", err)
	}

	// Add to traitor thread
	if game.TraitorThreadID != "" {
		notify.AddToThread(e.Session, game.TraitorThreadID, playerID)
		notify.SendThread(e.Session, game.TraitorThreadID,
			fmt.Sprintf("**%s** has been recruited by the host!", player.DiscordName))
	}

	// DM the new traitor
	traitors, err := db.GetPlayersByRole(e.DB, gameID, "traitor")
	if err != nil {
		slog.Error("force recruit: get traitors", "error", err, "game_id", gameID)
	}
	var others []string
	for _, t := range traitors {
		if t.DiscordID != playerID {
			others = append(others, t.DiscordName)
		}
	}
	msg := "The host has recruited you. You are now a **TRAITOR**!"
	if len(others) > 0 {
		msg += fmt.Sprintf("\nYour fellow traitors: %s", joinStrings(others))
	}
	notify.SendDM(e.Session, playerID, msg)

	// Clear recruitment if pending
	if game.RecruitmentPending {
		if err := db.SetRecruitmentPending(e.DB, gameID, false); err != nil {
			slog.Error("clear recruitment pending", "error", err)
		}
	}

	return nil
}

// sortStrings sorts a string slice in place.
func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// joinStrings joins strings with ", ".
func joinStrings(s []string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += ", "
		}
		result += v
	}
	return result
}
