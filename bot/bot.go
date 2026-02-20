package bot

import (
	"database/sql"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/bot/handlers"
	"github.com/gatorjuice/async_traitors/config"
)

// Bot is the Discord bot instance.
type Bot struct {
	Session *discordgo.Session
	DB      *sql.DB
	Config  *config.Config
}

// New creates a new Bot instance.
func New(cfg *config.Config, db *sql.DB) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, err
	}

	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsGuildMembers | discordgo.IntentsDirectMessages

	return &Bot{
		Session: session,
		DB:      db,
		Config:  cfg,
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

	slog.Info("bot started", "user", b.Session.State.User.Username, "guild", b.Config.GuildID)
	return nil
}

// Stop closes the Discord session.
func (b *Bot) Stop() {
	b.Session.Close()
}

func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	switch i.ApplicationCommandData().Name {
	case "create-game":
		handlers.HandleCreateGame(s, i, b.DB)
	case "join-game":
		handlers.HandleJoinGame(s, i, b.DB)
	case "start-game":
		handlers.HandleStartGame(s, i, b.DB)
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
		handlers.HandleAdvancePhase(s, i, b.DB)
	case "end-game":
		handlers.HandleEndGame(s, i, b.DB)
	case "help":
		handlers.HandleHelp(s, i, b.DB)
	}
}
