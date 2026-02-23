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

	// If a deadline is set, calculate and apply timers
	var deadlineWarning string
	if game.EndBy != "" {
		deadline, err := ParseDeadline(game.EndBy, game.HiatusTimezone)
		if err != nil {
			return fmt.Errorf("parse deadline: %w", err)
		}
		timers := CalculateTimersFromDeadline(time.Now(), deadline, len(players), game.HiatusStart, game.HiatusEnd, game.HiatusTimezone)
		if timers.IsTooTight {
			return fmt.Errorf("deadline is too short for %d players — need more time or fewer players", len(players))
		}
		if err := db.UpdateGameTimers(e.DB, gameID, timers.BreakfastMinutes, timers.RoundtableMinutes, timers.NightMinutes, timers.MissionMinutes); err != nil {
			return fmt.Errorf("update timers from deadline: %w", err)
		}
		if timers.IsTight {
			deadlineWarning = "\n\n**Warning:** Timers are tight for this deadline. Phases may feel rushed."
		}
	}

	// Create traitor thread
	thread, err := notify.CreateThread(e.Session, game.ChannelID, fmt.Sprintf("Traitors - Game #%d", gameID))
	if err != nil {
		slog.Error("create traitor thread", "error", err)
	} else {
		if err := db.SetTraitorThreadID(e.DB, gameID, thread.ID); err != nil {
			slog.Error("store traitor thread ID", "error", err)
		}

		traitors, err := db.GetPlayersByRole(e.DB, gameID, "traitor")
		if err != nil {
			slog.Error("start game: get traitors for thread", "error", err, "game_id", gameID)
		}
		for _, t := range traitors {
			notify.AddToThread(e.Session, thread.ID, t.DiscordID)
		}
		notify.SendThread(e.Session, thread.ID, "Welcome traitors! This is your private planning channel. Use it to coordinate your murders each night.")
	}

	// Update game state — starts with Breakfast
	if err := db.UpdateGameStatus(e.DB, gameID, string(StatusActive), string(PhaseBreakfast)); err != nil {
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
		fmt.Sprintf("Roles have been assigned and sent via DM.\n\n**%d players** | **%d traitor(s)** among you\n\nRound 1 begins with **Breakfast**!%s", len(players), traitorCount, deadlineWarning),
		notify.ColorSuccess,
		nil,
	)
	embed.Footer.Text = "Async Traitors | Round 1"
	notify.SendEmbed(e.Session, game.ChannelID, embed)

	// Reload game to get full state after updates
	game, err = db.GetGameByID(e.DB, gameID)
	if err != nil {
		slog.Error("start game: reload game", "error", err, "game_id", gameID)
	}

	// Start breakfast phase (round 1 has no murder to reveal)
	e.startBreakfastPhase(game)

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
	case PhaseBreakfast:
		nextPhase = PhaseMission
	case PhaseMission:
		nextPhase = PhaseRoundtable
	case PhaseRoundtable:
		// Tally votes before advancing
		banishedID, err := TallyBanishmentVotes(e.DB, e.Session, gameID, game.CurrentRound)
		if err != nil {
			slog.Error("tally banishment votes", "error", err)
		}

		// Check if a traitor was banished and trigger recruitment if needed
		if banishedID != "" {
			e.checkRecruitmentNeeded(gameID, banishedID)
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
		// Resolve night before advancing (silently — murder revealed at Breakfast)
		if !game.RecruitmentPending {
			if err := ResolveNight(e.DB, e.Session, gameID, game.CurrentRound); err != nil {
				slog.Error("resolve night", "error", err)
			}
		}
		// If recruitment was pending, it was already resolved via AcceptRecruitment/RefuseRecruitment

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
		nextPhase = PhaseBreakfast
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
	game, err = db.GetGameByID(e.DB, gameID)
	if err != nil {
		slog.Error("advance phase: reload game", "error", err, "game_id", gameID)
	}

	switch nextPhase {
	case PhaseBreakfast:
		e.startBreakfastPhase(game)
	case PhaseMission:
		e.startMissionPhase(game)
	case PhaseRoundtable:
		e.startRoundtablePhase(game)
	case PhaseNight:
		e.startNightPhase(game)
	}

	return nil
}

// CheckWinCondition checks if the game has ended.
func (e *Engine) CheckWinCondition(gameID int64) (bool, string, error) {
	game, err := db.GetGameByID(e.DB, gameID)
	if err != nil {
		return false, "", err
	}

	traitors, err := db.GetPlayersByRole(e.DB, gameID, "traitor")
	if err != nil {
		return false, "", err
	}

	faithful, err := db.GetPlayersByRole(e.DB, gameID, "faithful")
	if err != nil {
		return false, "", err
	}

	// Don't end if recruitment is pending — traitors will be replenished
	if len(traitors) == 0 && !game.RecruitmentPending {
		return true, "faithful", nil
	}

	if len(traitors) >= len(faithful) {
		return true, "traitors", nil
	}

	return false, "", nil
}

// checkRecruitmentNeeded checks if recruitment should trigger after a banishment.
func (e *Engine) checkRecruitmentNeeded(gameID int64, banishedID string) {
	banished, err := db.GetPlayer(e.DB, gameID, banishedID)
	if err != nil || banished.Role != string(RoleTraitor) {
		return // Only trigger recruitment when a traitor is banished
	}

	traitors, err := db.GetPlayersByRole(e.DB, gameID, "traitor")
	if err != nil {
		return
	}

	alive, err := db.GetAlivePlayers(e.DB, gameID)
	if err != nil {
		return
	}

	game, err := db.GetGameByID(e.DB, gameID)
	if err != nil {
		return
	}

	// Only trigger recruitment if traitors < 2 and not at endgame threshold
	if len(traitors) < 2 && len(alive) > game.RevealThreshold {
		if err := db.SetRecruitmentPending(e.DB, gameID, true); err != nil {
			slog.Error("set recruitment pending", "error", err)
		}
		slog.Info("recruitment triggered", "game", gameID, "remaining_traitors", len(traitors))
	}
}

func (e *Engine) startBreakfastPhase(game *db.Game) {
	// Reveal murder from previous night (if not round 1)
	if game.CurrentRound > 1 {
		prevRound := game.CurrentRound - 1
		e.revealMurderAtBreakfast(game, prevRound)
	} else {
		// Round 1 — no murder to reveal
		embed := notify.GameEmbed(
			"Breakfast",
			fmt.Sprintf("Round %d — The players gather for Breakfast.\n\nNo one was murdered last night... but the traitors are among you.\nTimer: %d minutes", game.CurrentRound, game.TimerBreakfastMinutes),
			notify.ColorWarning,
			nil,
		)
		embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
		notify.SendEmbed(e.Session, game.ChannelID, embed)
	}

	e.startPhaseTimer(game, game.TimerBreakfastMinutes)
	e.scheduleWarnings(game, game.TimerBreakfastMinutes, func(remaining int) {
		notify.SendChannel(e.Session, game.ChannelID,
			fmt.Sprintf("Breakfast — %d minutes remaining!", remaining))
	})
}

func (e *Engine) revealMurderAtBreakfast(game *db.Game, prevRound int) {
	murdered, err := db.GetPlayersByStatusAndRound(e.DB, game.ID, "murdered", prevRound)
	if err != nil {
		slog.Error("breakfast: get murdered players", "error", err, "game_id", game.ID, "round", prevRound)
	}

	// Check if a shield was used
	shieldLog, err := db.GetShieldLog(e.DB, game.ID)
	if err != nil {
		slog.Error("breakfast: get shield log", "error", err, "game_id", game.ID)
	}
	shieldUsed := false
	for _, entry := range shieldLog {
		if entry.RoundUsed != nil && *entry.RoundUsed == prevRound {
			shieldUsed = true
			break
		}
	}

	if len(murdered) > 0 {
		// Dramatic reveal
		notify.SendChannel(e.Session, game.ChannelID, "The players gather for Breakfast... but someone is missing.")
		if e.Session != nil {
			time.Sleep(3 * time.Second)
		}

		for _, victim := range murdered {
			embed := notify.GameEmbed(
				"Murder!",
				fmt.Sprintf("**%s** was found murdered...", victim.DiscordName),
				notify.ColorNight,
				nil,
			)
			embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
			notify.SendEmbed(e.Session, game.ChannelID, embed)

			if err := RevealRole(e.DB, e.Session, game.ID, victim.DiscordID); err != nil {
				slog.Error("reveal role at breakfast", "error", err)
			}
		}
	} else if shieldUsed {
		notify.SendChannel(e.Session, game.ChannelID, "The players gather for Breakfast...")
		if e.Session != nil {
			time.Sleep(2 * time.Second)
		}
		embed := notify.GameEmbed(
			"Shield Block!",
			"The traitors struck, but their target was protected by a shield!",
			notify.ColorNight,
			nil,
		)
		embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
		notify.SendEmbed(e.Session, game.ChannelID, embed)
	} else {
		embed := notify.GameEmbed(
			"Breakfast",
			fmt.Sprintf("Round %d — Everyone made it through the night. A peaceful morning.", game.CurrentRound),
			notify.ColorWarning,
			nil,
		)
		embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
		notify.SendEmbed(e.Session, game.ChannelID, embed)
	}

	// After the reveal, announce the Breakfast phase timer
	embed := notify.GameEmbed(
		"Breakfast",
		fmt.Sprintf("Round %d — Time to discuss! Who do you trust? Who seems suspicious?\nTimer: %d minutes", game.CurrentRound, game.TimerBreakfastMinutes),
		notify.ColorWarning,
		nil,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
	notify.SendEmbed(e.Session, game.ChannelID, embed)
}

func (e *Engine) startMissionPhase(game *db.Game) {
	embed := notify.GameEmbed(
		"Mission Phase",
		fmt.Sprintf("Round %d — Mission time!\n\nThe game admin should start a mission with `/start-mission`.\nTimer: %d minutes", game.CurrentRound, game.TimerMissionMinutes),
		notify.ColorInfo,
		nil,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
	notify.SendEmbed(e.Session, game.ChannelID, embed)
	e.startPhaseTimer(game, game.TimerMissionMinutes)
	e.scheduleWarnings(game, game.TimerMissionMinutes, func(remaining int) {
		notify.SendChannel(e.Session, game.ChannelID,
			fmt.Sprintf("Mission phase — %d minutes remaining!", remaining))
	})
}

func (e *Engine) startRoundtablePhase(game *db.Game) {
	alive, err := db.GetAlivePlayers(e.DB, game.ID)
	if err != nil {
		slog.Error("roundtable: get alive players", "error", err, "game_id", game.ID)
	}
	var playerList string
	for _, p := range alive {
		playerList += fmt.Sprintf("• %s\n", p.DiscordName)
	}

	embed := notify.GameEmbed(
		"Round Table",
		fmt.Sprintf("Round %d — The Round Table convenes!\n\nUse `/vote player:@name` to cast your vote. Votes are secret — results will be revealed after everyone has voted or the timer expires.\nTimer: %d minutes\n\n**Alive players:**\n%s", game.CurrentRound, game.TimerRoundtableMinutes, playerList),
		notify.ColorDanger,
		nil,
	)
	embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
	notify.SendEmbed(e.Session, game.ChannelID, embed)
	e.startPhaseTimer(game, game.TimerRoundtableMinutes)
	e.scheduleWarnings(game, game.TimerRoundtableMinutes, func(remaining int) {
		votes, err := db.GetVotes(e.DB, game.ID, game.CurrentRound, "roundtable")
		if err != nil {
			slog.Error("roundtable warning: get votes", "error", err, "game_id", game.ID)
		}
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
	// Reload game to check recruitment status
	game, err := db.GetGameByID(e.DB, game.ID)
	if err != nil {
		slog.Error("night: reload game", "error", err, "game_id", game.ID)
	}

	if game.RecruitmentPending {
		// Recruitment night
		embed := notify.GameEmbed(
			"Night Falls...",
			fmt.Sprintf("Round %d — The players go to sleep.\n\nSomething stirs in the shadows...\nTimer: %d minutes", game.CurrentRound, game.TimerNightMinutes),
			notify.ColorNight,
			nil,
		)
		embed.Footer.Text = fmt.Sprintf("Async Traitors | Round %d", game.CurrentRound)
		notify.SendEmbed(e.Session, game.ChannelID, embed)

		// Prompt traitors for recruitment in thread
		if game.TraitorThreadID != "" {
			alive, err := db.GetAlivePlayers(e.DB, game.ID)
			if err != nil {
				slog.Error("recruitment night: get alive players", "error", err, "game_id", game.ID)
			}
			var targets string
			for _, p := range alive {
				if p.Role != "traitor" {
					targets += fmt.Sprintf("• %s\n", p.DiscordName)
				}
			}
			notify.SendThread(e.Session, game.TraitorThreadID,
				fmt.Sprintf("**Recruitment Night %d** — You've lost an ally. Choose a faithful player to recruit to your cause.\n\nUse `/recruit player:@name` to make your choice.\n\n**Available targets:**\n%s", game.CurrentRound, targets))
		}
	} else {
		// Normal murder night
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
			alive, err := db.GetAlivePlayers(e.DB, game.ID)
			if err != nil {
				slog.Error("murder night: get alive players", "error", err, "game_id", game.ID)
			}
			var targets string
			for _, p := range alive {
				if p.Role != "traitor" {
					targets += fmt.Sprintf("• %s\n", p.DiscordName)
				}
			}
			notify.SendThread(e.Session, game.TraitorThreadID,
				fmt.Sprintf("**Night %d** — Choose your victim!\n\nUse `/murder-vote player:@name` to vote.\n\n**Available targets:**\n%s", game.CurrentRound, targets))
		}
	}

	e.startPhaseTimer(game, game.TimerNightMinutes)
	e.scheduleWarnings(game, game.TimerNightMinutes, func(remaining int) {
		traitors, err := db.GetPlayersByRole(e.DB, game.ID, "traitor")
		if err != nil {
			slog.Error("night warning: get traitors", "error", err, "game_id", game.ID)
		}
		votes, err := db.GetVotes(e.DB, game.ID, game.CurrentRound, "night")
		if err != nil {
			slog.Error("night warning: get votes", "error", err, "game_id", game.ID)
		}
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

	banished, err := db.GetPlayersByStatusAndRound(e.DB, gameID, "banished", completedRound)
	if err != nil {
		slog.Error("round recap: get banished", "error", err, "game_id", gameID, "round", completedRound)
	}
	murdered, err := db.GetPlayersByStatusAndRound(e.DB, gameID, "murdered", completedRound)
	if err != nil {
		slog.Error("round recap: get murdered", "error", err, "game_id", gameID, "round", completedRound)
	}
	alive, err := db.GetAlivePlayers(e.DB, gameID)
	if err != nil {
		slog.Error("round recap: get alive players", "error", err, "game_id", gameID)
	}
	shieldLog, err := db.GetShieldLog(e.DB, gameID)
	if err != nil {
		slog.Error("round recap: get shield log", "error", err, "game_id", gameID)
	}

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
	alive, err := db.GetAlivePlayers(e.DB, gameID)
	if err != nil {
		slog.Error("end game: get alive players", "error", err, "game_id", gameID)
	}
	var roleList string
	for _, p := range alive {
		roleName := "FAITHFUL"
		if p.Role == "traitor" {
			roleName = "TRAITOR"
		}
		roleList += fmt.Sprintf("• **%s** — %s\n", p.DiscordName, roleName)
	}

	// Build final stats
	allPlayers, err := db.GetAllPlayers(e.DB, gameID)
	if err != nil {
		slog.Error("end game: get all players", "error", err, "game_id", gameID)
	}
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

	// Add buy-in payout info if applicable.
	if game.BuyinAmount > 0 {
		potTotal := len(allPlayers) * game.BuyinAmount
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Prize Pool",
			Value:  FormatCents(potTotal),
			Inline: true,
		})

		winners, losers := CalculatePayouts(allPlayers, winner, game.BuyinAmount)

		var winnerNames string
		for _, w := range winners {
			winnerNames += fmt.Sprintf("• **%s**\n", w.PlayerName)
		}
		if winnerNames != "" {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  "Winners",
				Value: winnerNames,
			})
		}

		// DM winners to share payment info.
		for _, w := range winners {
			notify.SendDM(e.Session, w.PlayerDiscordID,
				fmt.Sprintf("Congratulations, you won! You'll receive **%s** from each losing player.\n\nUse `/wallet info:your-payment-info` in the game channel to share your payment details with losers.",
					FormatCents(game.BuyinAmount/len(winners))))
		}

		// DM losers with amount owed.
		for _, l := range losers {
			notify.SendDM(e.Session, l.PlayerDiscordID,
				fmt.Sprintf("Game over! You owe **%s** total. Winners will share their payment info shortly via `/wallet`.",
					FormatCents(l.Amount)))
		}
	}

	embed := notify.GameEmbed(title, description, color, fields)
	embed.Footer.Text = "Async Traitors | Game Over"
	notify.SendEmbed(e.Session, game.ChannelID, embed)

	return nil
}
