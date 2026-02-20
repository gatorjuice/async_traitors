package bot

import (
	"database/sql"
	"log/slog"
	"runtime/debug"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/bot/handlers"
	"github.com/gatorjuice/async_traitors/config"
	"github.com/gatorjuice/async_traitors/game"
)

// Bot is the Discord bot instance.
type Bot struct {
	Session *discordgo.Session
	DB      *sql.DB
	Config  *config.Config
	Engine  *game.Engine
}

// New creates a new Bot instance.
func New(cfg *config.Config, db *sql.DB) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, err
	}

	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuildMembers | discordgo.IntentsDirectMessages

	engine := game.NewEngine(db, session)

	return &Bot{
		Session: session,
		DB:      db,
		Config:  cfg,
		Engine:  engine,
	}, nil
}

// Start opens the Discord session and registers commands.
func (b *Bot) Start() error {
	b.Session.AddHandler(b.handleInteraction)

	if err := b.Session.Open(); err != nil {
		return err
	}

	_, err := b.Session.ApplicationCommandBulkOverwrite(b.Session.State.User.ID, b.Config.GuildID, Commands)
	if err != nil {
		return err
	}

	// Detect active games from before restart
	b.detectActiveGames()

	slog.Info("bot started", "user", b.Session.State.User.Username, "guild", b.Config.GuildID)
	return nil
}

// Stop closes the Discord session.
func (b *Bot) Stop() {
	b.Session.Close()
}

func (b *Bot) detectActiveGames() {
	rows, err := b.DB.Query("SELECT id, channel_id, current_phase, current_round FROM games WHERE status = 'active'")
	if err != nil {
		slog.Error("detect active games", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var channelID, phase string
		var round int
		if err := rows.Scan(&id, &channelID, &phase, &round); err != nil {
			continue
		}
		slog.Warn("active game detected on startup — timers not restored",
			"game_id", id, "channel", channelID, "phase", phase, "round", round)
	}
}

func recoverHandler(name string, handler func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in handler", "command", name, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	handler()
}

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	name := i.ApplicationCommandData().Name
	recoverHandler(name, func() {
		switch name {
		case "create-game":
			handlers.HandleCreateGame(s, i, b.DB)
		case "join-game":
			handlers.HandleJoinGame(s, i, b.DB)
		case "start-game":
			handlers.HandleStartGame(s, i, b.DB, b.Engine)
		case "my-role":
			handlers.HandleMyRole(s, i, b.DB)
		case "game-info":
			handlers.HandleGameInfo(s, i, b.DB)
		case "players":
			handlers.HandlePlayers(s, i, b.DB)
		case "vote":
			handlers.HandleVote(s, i, b.DB)
		case "murder-vote":
			handlers.HandleMurderVote(s, i, b.DB)
		case "claim-shield":
			handlers.HandleClaimShield(s, i, b.DB)
		case "start-competition":
			handlers.HandleStartCompetition(s, i, b.DB)
		case "submit-answer":
			handlers.HandleSubmitAnswer(s, i, b.DB)
		case "end-competition":
			handlers.HandleEndCompetition(s, i, b.DB)
		case "grant-shield":
			handlers.HandleGrantShield(s, i, b.DB)
		case "set-timers":
			handlers.HandleSetTimers(s, i, b.DB)
		case "advance-phase":
			handlers.HandleAdvancePhase(s, i, b.DB, b.Engine)
		case "end-game":
			handlers.HandleEndGame(s, i, b.DB)
		case "help":
			handlers.HandleHelp(s, i, b.DB)
		}
	})
}
