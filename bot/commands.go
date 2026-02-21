package bot

import "github.com/bwmarrin/discordgo"

// Commands defines all 17 slash commands for the bot.
var Commands = []*discordgo.ApplicationCommand{
	{
		Name:        "create-game",
		Description: "Create a new Traitors game in this channel",
	},
	{
		Name:        "join-game",
		Description: "Join a game with a code",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "code",
				Description: "The join code for the game",
				Required:    true,
			},
		},
	},
	{
		Name:        "start-game",
		Description: "Start the game (admin only)",
	},
	{
		Name:        "my-role",
		Description: "Check your secret role",
	},
	{
		Name:        "game-info",
		Description: "Show current game status",
	},
	{
		Name:        "players",
		Description: "List all players and status",
	},
	{
		Name:        "vote",
		Description: "Vote to banish a player",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "player",
				Description: "The player to vote against",
				Required:    true,
			},
		},
	},
	{
		Name:        "murder-vote",
		Description: "Vote to murder a player (traitors only)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "player",
				Description: "The player to target for murder",
				Required:    true,
			},
		},
	},
	{
		Name:        "claim-shield",
		Description: "Claim you won a shield (honor system)",
	},
	{
		Name:        "start-competition",
		Description: "Start a competition round (admin)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "type",
				Description: "The type of competition",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "trivia", Value: "trivia"},
					{Name: "speed", Value: "speed"},
					{Name: "puzzle", Value: "puzzle"},
					{Name: "scavenger", Value: "scavenger"},
				},
			},
		},
	},
	{
		Name:        "submit-answer",
		Description: "Submit your competition answer",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "answer",
				Description: "Your answer to the competition question",
				Required:    true,
			},
		},
	},
	{
		Name:        "end-competition",
		Description: "End current competition (admin)",
	},
	{
		Name:        "grant-shield",
		Description: "Grant a shield to a player (admin)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "player",
				Description: "The player to grant a shield to",
				Required:    true,
			},
		},
	},
	{
		Name:        "set-timers",
		Description: "Configure phase timers in minutes (admin)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "discussion",
				Description: "Discussion phase timer in minutes",
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "voting",
				Description: "Voting phase timer in minutes",
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "night",
				Description: "Night phase timer in minutes",
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "competition",
				Description: "Competition phase timer in minutes",
			},
		},
	},
	{
		Name:        "advance-phase",
		Description: "Manually advance to next phase (admin)",
	},
	{
		Name:        "end-game",
		Description: "Force-end the game (admin)",
	},
	{
		Name:        "help",
		Description: "Show help and game rules",
	},
	{
		Name:        "set-hiatus",
		Description: "Set quiet hours — timers pause during this window (admin)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "start",
				Description: "Start time in HH:MM (24h format)",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "end",
				Description: "End time in HH:MM (24h format)",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "timezone",
				Description: "IANA timezone name (e.g. America/New_York, Europe/London, UTC)",
				Required:    true,
			},
		},
	},
	{
		Name:        "recap",
		Description: "Show the game timeline so far",
	},
}
