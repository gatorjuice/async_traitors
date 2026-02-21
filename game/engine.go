package game

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/notify"
)

// Engine manages the core game loop and phase transitions.
type Engine struct {
	DB      *sql.DB
	Session *discordgo.Session
	Timers  *TimerManager
}

// NewEngine creates a new game engine.
func NewEngine(database *sql.DB, session *discordgo.Session) *Engine {
	return &Engine{
		DB:      database,
		Session: session,
		Timers:  NewTimerManager(),
	}
}

// StartGame initiates a game from the lobby.
func (e *Engine) StartGame(gameID int64) error {
	game, err := db.GetGameByID(e.DB, gameID)
	if err != nil {
		return err
	}

	if game.Status != string(StatusLobby) {
		return errors.New("game is not in lobby")
	}

	players, err := db.GetAllPlayers(e.DB, gameID)
	if err != nil {
		return err
	}

	if len(players) < 4 {
		return fmt.Errorf("need at least 4 players, currently have %d", len(players))
	}

	// Assign roles
	if err := AssignRoles(e.DB, e.Session, gameID); err != nil {
		return fmt.Errorf("assign roles: %w", err)
	}

	// Create traitor thread
	thread, err := notify.CreateThread(e.Session, game.ChannelID, fmt.Sprintf("Traitors - Game #%d", gameID))
	if err != nil {
		slog.Error("create traitor thread", "error", err)
	} else {
		if err := db.SetTraitorThreadID(e.DB, gameID, thread.ID); err != nil {
			slog.Error("store traitor thread ID", "error", err)
		}

		traitors, _ := db.GetPlayersByRole(e.DB, gameID, "traitor")
		for _, t := range traitors {
			notify.AddToThread(e.Session, thread.ID, t.DiscordID)
		}
		notify.SendThread(e.Session, thread.ID, "Welcome traitors! This is your private planning channel. Use it to coordinate your murders each night.")
	}

	// Update game state
	if err := db.UpdateGameStatus(e.DB, gameID, string(StatusActive), string(PhaseCompetition)); err != nil {
		return err
	}
	if err := db.UpdateGameRound(e.DB, gameID, 1); err != nil {
		return err
	}

	// Announce game start
	traitorCount := len(players) / 4
	if traitorCount < 1 {
		traitorCount = 1
	}

	embed := notify.GameEmbed(
		"The Game Begins!",
		fmt.Sprintf("Roles have been assigned and sent via DM.\n\n**%d players** | **%d traitor(s)** among you\n\nRound 1 begins with the **Competition** phase!", len(players), traitorCount),
		notify.ColorSuccess,
		nil,
	)
	embed.Footer.Text = "Async Traitors | Round 1"
	notify.SendEmbed(e.Session, game.ChannelID, embed)

	// Reload game to get full state after updates
	game, _ = db.GetGameByID(e.DB, gameID)

	// Start competition timer
	e.startPhaseTimer(game, game.TimerCompetitionMinutes)

	return nil
}

// AdvancePhase transitions the game to the next phase.
func (e *Engine) AdvancePhase(gameID int64) error {
	game, err := db.GetGameByID(e.DB, gameID)
	if err != nil {
		return err
	}

	if game.Status != string(StatusActive) {
		return errors.New("game is not active")
	}

	e.Timers.CancelTimer(gameID)

	currentPhase := Phase(game.CurrentPhase)
	var nextPhase Phase

	switch currentPhase {
	case PhaseCompetition:
		nextPhase = PhaseDiscussion
	case PhaseDiscussion:
		nextPhase = PhaseVoting
	case PhaseVoting:
		// Tally votes before advancing
		_, err := TallyBanishmentVotes(e.DB, e.Session, gameID, game.CurrentRound)
		if err != nil {
			slog.Error("tally banishment votes", "error", err)
		}

		// Check win condition
		finished, winner, err := e.CheckWinCondition(gameID)
		if err != nil {
			slog.Error("check win condition", "error", err)
		}
		if finished {
			return e.endGame(gameID, winner, game.CurrentRound)
		}

		nextPhase = PhaseNight
	case PhaseNight:
		// Resolve night before advancing
		if err := ResolveNight(e.DB, e.Session, gameID, game.CurrentRound); err != nil {
			slog.Error("resolve night", "error", err)
		}

		// Check win condition
		finished, winner, err := e.CheckWinCondition(gameID)
		if err != nil {
			slog.Error("check win condition", "error", err)
		}
		if finished {
			return e.endGame(gameID, winner, game.CurrentRound)
		}

		// Post round recap before starting next round
		e.postRoundRecap(gameID, game.CurrentRound)

		// Start next round
		newRound := game.CurrentRound + 1
		if err := db.UpdateGameRound(e.DB, gameID, newRound); err != nil {
			return err
		}
		nextPhase = PhaseCompetition
	default:
		return fmt.Errorf("cannot advance from phase: %s", currentPhase)
	}

	if !CanTransition(currentPhase, nextPhase) {
		return fmt.Errorf("invalid transition from %s to %s", currentPhase, nextPhase)
	}

	if err := db.UpdateGamePhase(e.DB, gameID, string(nextPhase)); err != nil {
		return err
	}

	// Reload game for updated round
	game, _ = db.GetGameByID(e.DB, gameID)

	switch nextPhase {
	case PhaseCompetition:
		e.startCompetitionPhase(game)
	case PhaseDiscussion:
		e.startDiscussionPhase(game)
	case PhaseVoting:
		e.startVotingPhase(game)
	case PhaseNight:
		e.startNightPhase(game)
	}

	return nil
}

// CheckWinCondition checks if the game has ended.
func (e *Engine) CheckWinCondition(gameID int64) (bool, string, error) {
	traitors, err := db.GetPlayersByRole(e.DB, gameID, "traitor")
	if err != nil {
		return false, "", err
	}

	faithful, err := db.GetPlayersByRole(e.DB, gameID, "faithful")
	if err != nil {
		return false, "", err
	}

	if len(traitors) == 0 {
		return true, "faithful", nil
	}

	if len(traitors) >= len(faithful) {
		return true, "traitors", nil
	}

	return false, "", nil
}

func (e *Engine) startCompetitionPhase(game *db.Game) {
	embed := notify.GameEmbed(
		"Competition Phase",
		fmt.Sprintf("Round %d — Competition time!\n\nThe game admin should start a competition with `/start-competition`.\nTimer: %d minutes", game.CurrentRound, game.TimerCompetitionMinutes),
		notify.ColorInfo,
		nil,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
	notify.SendEmbed(e.Session, game.ChannelID, embed)
	e.startPhaseTimer(game, game.TimerCompetitionMinutes)
	e.scheduleWarnings(game, game.TimerCompetitionMinutes, func(remaining int) {
		notify.SendChannel(e.Session, game.ChannelID,
			fmt.Sprintf("Competition phase — %d minutes remaining!", remaining))
	})
}

func (e *Engine) startDiscussionPhase(game *db.Game) {
	embed := notify.GameEmbed(
		"Discussion Phase",
		fmt.Sprintf("Round %d — Time to discuss!\n\nTalk it out. Who do you trust? Who seems suspicious?\nTimer: %d minutes", game.CurrentRound, game.TimerDiscussionMinutes),
		notify.ColorWarning,
		nil,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
	notify.SendEmbed(e.Session, game.ChannelID, embed)
	e.startPhaseTimer(game, game.TimerDiscussionMinutes)
	e.scheduleWarnings(game, game.TimerDiscussionMinutes, func(remaining int) {
		notify.SendChannel(e.Session, game.ChannelID,
			fmt.Sprintf("Discussion phase — %d minutes remaining!", remaining))
	})
}

func (e *Engine) startVotingPhase(game *db.Game) {
	alive, _ := db.GetAlivePlayers(e.DB, game.ID)
	var playerList string
	for _, p := range alive {
		playerList += fmt.Sprintf("• %s\n", p.DiscordName)
	}

	embed := notify.GameEmbed(
		"Voting Phase",
		fmt.Sprintf("Round %d — Time to vote!\n\nUse `/vote player:@name` to cast your vote. Votes are secret — results will be revealed after everyone has voted or the timer expires.\nTimer: %d minutes\n\n**Alive players:**\n%s", game.CurrentRound, game.TimerVotingMinutes, playerList),
		notify.ColorDanger,
		nil,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
	notify.SendEmbed(e.Session, game.ChannelID, embed)
	e.startPhaseTimer(game, game.TimerVotingMinutes)
	e.scheduleWarnings(game, game.TimerVotingMinutes, func(remaining int) {
		votes, _ := db.GetVotes(e.DB, game.ID, game.CurrentRound, "voting")
		voted := make(map[string]bool, len(votes))
		for _, v := range votes {
			voted[v.VoterDiscordID] = true
		}
		nonVoters := 0
		for _, p := range alive {
			if !voted[p.DiscordID] {
				nonVoters++
				notify.SendDM(e.Session, p.DiscordID,
					fmt.Sprintf("Reminder: %d minutes left to vote! Use `/vote` in the game channel.", remaining))
			}
		}
		if nonVoters > 0 {
			notify.SendChannel(e.Session, game.ChannelID,
				fmt.Sprintf("%d player(s) still need to vote — %d minutes remaining!", nonVoters, remaining))
		}
	})
}

func (e *Engine) startNightPhase(game *db.Game) {
	embed := notify.GameEmbed(
		"Night Falls...",
		fmt.Sprintf("Round %d — The players go to sleep.\n\nTraitors, check your private thread to choose your victim.\nTimer: %d minutes", game.CurrentRound, game.TimerNightMinutes),
		notify.ColorNight,
		nil,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
	notify.SendEmbed(e.Session, game.ChannelID, embed)

	// Prompt traitors in thread
	if game.TraitorThreadID != "" {
		alive, _ := db.GetAlivePlayers(e.DB, game.ID)
		var targets string
		for _, p := range alive {
			if p.Role != "traitor" {
				targets += fmt.Sprintf("• %s\n", p.DiscordName)
			}
		}
		notify.SendThread(e.Session, game.TraitorThreadID,
			fmt.Sprintf("**Night %d** — Choose your victim!\n\nUse `/murder-vote player:@name` to vote.\n\n**Available targets:**\n%s", game.CurrentRound, targets))
	}

	e.startPhaseTimer(game, game.TimerNightMinutes)
	e.scheduleWarnings(game, game.TimerNightMinutes, func(remaining int) {
		traitors, _ := db.GetPlayersByRole(e.DB, game.ID, "traitor")
		votes, _ := db.GetVotes(e.DB, game.ID, game.CurrentRound, "night")
		voted := make(map[string]bool, len(votes))
		for _, v := range votes {
			voted[v.VoterDiscordID] = true
		}
		nonVoters := 0
		for _, t := range traitors {
			if !voted[t.DiscordID] {
				nonVoters++
				notify.SendDM(e.Session, t.DiscordID,
					fmt.Sprintf("Reminder: %d minutes left for night phase! Use `/murder-vote` to choose your victim.", remaining))
			}
		}
		if nonVoters > 0 && game.TraitorThreadID != "" {
			notify.SendThread(e.Session, game.TraitorThreadID,
				fmt.Sprintf("%d traitor(s) still need to vote — %d minutes remaining!", nonVoters, remaining))
		}
	})
}

func (e *Engine) startPhaseTimer(g *db.Game, minutes int) {
	active := time.Duration(minutes) * time.Minute
	wall := EffectiveWallDuration(time.Now(), active, g.HiatusStart, g.HiatusEnd, g.HiatusTimezone)
	e.Timers.StartTimer(g.ID, wall, func() {
		// If timer fires during hiatus, wait until it ends.
		if IsInHiatus(g.HiatusStart, g.HiatusEnd, g.HiatusTimezone, time.Now()) {
			wait := TimeUntilHiatusEnd(g.HiatusStart, g.HiatusEnd, g.HiatusTimezone, time.Now())
			slog.Info("phase timer waiting for hiatus to end", "game", g.ID, "wait", wait)
			time.Sleep(wait)
		}
		slog.Info("phase timer expired", "game", g.ID)
		if err := e.AdvancePhase(g.ID); err != nil {
			slog.Error("auto-advance phase", "error", err, "game", g.ID)
		}
	})
}

// scheduleWarnings schedules warning callbacks at halfway and 5-minutes-remaining.
// warningFn receives the remaining minutes as its argument.
func (e *Engine) scheduleWarnings(g *db.Game, activeMinutes int, warningFn func(remaining int)) {
	if activeMinutes <= 10 {
		// Only schedule 5-min warning if the phase is long enough.
		if activeMinutes > 5 {
			delay := EffectiveWallDuration(time.Now(), time.Duration(activeMinutes-5)*time.Minute, g.HiatusStart, g.HiatusEnd, g.HiatusTimezone)
			e.Timers.ScheduleCallback(g.ID, delay, func() {
				if !IsInHiatus(g.HiatusStart, g.HiatusEnd, g.HiatusTimezone, time.Now()) {
					warningFn(5)
				}
			})
		}
		return
	}

	// Halfway warning
	halfDelay := EffectiveWallDuration(time.Now(), time.Duration(activeMinutes/2)*time.Minute, g.HiatusStart, g.HiatusEnd, g.HiatusTimezone)
	e.Timers.ScheduleCallback(g.ID, halfDelay, func() {
		if !IsInHiatus(g.HiatusStart, g.HiatusEnd, g.HiatusTimezone, time.Now()) {
			warningFn(activeMinutes - activeMinutes/2)
		}
	})

	// 5-minute warning
	fiveDelay := EffectiveWallDuration(time.Now(), time.Duration(activeMinutes-5)*time.Minute, g.HiatusStart, g.HiatusEnd, g.HiatusTimezone)
	e.Timers.ScheduleCallback(g.ID, fiveDelay, func() {
		if !IsInHiatus(g.HiatusStart, g.HiatusEnd, g.HiatusTimezone, time.Now()) {
			warningFn(5)
		}
	})
}

// postRoundRecap posts a summary of what happened during the completed round.
func (e *Engine) postRoundRecap(gameID int64, completedRound int) {
	g, err := db.GetGameByID(e.DB, gameID)
	if err != nil {
		return
	}

	banished, _ := db.GetPlayersByStatusAndRound(e.DB, gameID, "banished", completedRound)
	murdered, _ := db.GetPlayersByStatusAndRound(e.DB, gameID, "murdered", completedRound)
	alive, _ := db.GetAlivePlayers(e.DB, gameID)
	shieldLog, _ := db.GetShieldLog(e.DB, gameID)

	var desc string
	if len(banished) > 0 {
		for _, p := range banished {
			roleName := "FAITHFUL"
			if p.Role == "traitor" {
				roleName = "TRAITOR"
			}
			desc += fmt.Sprintf("Banished: **%s** (%s)\n", p.DiscordName, roleName)
		}
	} else {
		desc += "No one was banished.\n"
	}

	if len(murdered) > 0 {
		for _, p := range murdered {
			roleName := "FAITHFUL"
			if p.Role == "traitor" {
				roleName = "TRAITOR"
			}
			desc += fmt.Sprintf("Murdered: **%s** (%s)\n", p.DiscordName, roleName)
		}
	} else {
		// Check if a shield was used this round
		shieldUsed := false
		for _, entry := range shieldLog {
			if entry.RoundUsed != nil && *entry.RoundUsed == completedRound {
				shieldUsed = true
				break
			}
		}
		if shieldUsed {
			desc += "A shield blocked the traitors' attack!\n"
		} else {
			desc += "No one was murdered. A peaceful night.\n"
		}
	}

	// Count active shields
	activeShields := 0
	for _, p := range alive {
		if p.HasShield {
			activeShields++
		}
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "Players Remaining", Value: fmt.Sprintf("%d", len(alive)), Inline: true},
		{Name: "Active Shields", Value: fmt.Sprintf("%d", activeShields), Inline: true},
	}

	embed := notify.GameEmbed(
		fmt.Sprintf("Round %d Recap", completedRound),
		desc,
		notify.ColorInfo,
		fields,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d Complete", completedRound)
	notify.SendEmbed(e.Session, g.ChannelID, embed)
}

func (e *Engine) endGame(gameID int64, winner string, round int) error {
	e.Timers.CancelTimer(gameID)

	game, err := db.GetGameByID(e.DB, gameID)
	if err != nil {
		return err
	}

	if err := db.UpdateGameStatus(e.DB, gameID, string(StatusFinished), "finished"); err != nil {
		return err
	}

	var title, description string
	var color int
	if winner == "faithful" {
		title = "Faithful Win!"
		description = "The faithful have won! All traitors have been banished!"
		color = notify.ColorSuccess
	} else {
		title = "Traitors Win!"
		description = "The traitors have won! They outnumber the faithful!"
		color = notify.ColorDanger
	}

	// Reveal all remaining roles
	alive, _ := db.GetAlivePlayers(e.DB, gameID)
	var roleList string
	for _, p := range alive {
		roleName := "FAITHFUL"
		if p.Role == "traitor" {
			roleName = "TRAITOR"
		}
		roleList += fmt.Sprintf("• **%s** — %s\n", p.DiscordName, roleName)
	}

	// Build final stats
	allPlayers, _ := db.GetAllPlayers(e.DB, gameID)
	var stats string
	for _, p := range allPlayers {
		roleName := "FAITHFUL"
		if p.Role == "traitor" {
			roleName = "TRAITOR"
		}
		status := "Alive"
		switch p.Status {
		case "banished":
			status = "Banished"
		case "murdered":
			status = "Murdered"
		}
		stats += fmt.Sprintf("• **%s** — %s (%s)\n", p.DiscordName, roleName, status)
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "Rounds Played", Value: fmt.Sprintf("%d", round), Inline: true},
		{Name: "Surviving Players", Value: roleList, Inline: false},
		{Name: "All Players", Value: stats, Inline: false},
	}

	embed := notify.GameEmbed(title, description, color, fields)
	embed.Footer.Text = "Async Traitors | Game Over"
	notify.SendEmbed(e.Session, game.ChannelID, embed)

	return nil
}
