package bot

import "github.com/bwmarrin/discordgo"

// Commands defines all slash commands for the bot.
var Commands = []*discordgo.ApplicationCommand{
	{
		Name:        "create-game",
		Description: "Create a new Traitors game in this channel",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "buyin",
				Description: "Buy-in amount in dollars (e.g. 5 or 10.50)",
			},
		},
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
		Description: "Vote to banish a player at the Round Table",
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
		Name:        "recruit",
		Description: "Choose a player to recruit (traitors only, recruitment night)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "player",
				Description: "The faithful player to recruit",
				Required:    true,
			},
		},
	},
	{
		Name:        "accept-recruitment",
		Description: "Accept the traitors' offer to join them",
	},
	{
		Name:        "refuse-recruitment",
		Description: "Refuse the traitors' offer (you will be murdered)",
	},
	{
		Name:        "claim-shield",
		Description: "Claim you won a shield (honor system)",
	},
	{
		Name:        "start-mission",
		Description: "Start a mission (admin)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "type",
				Description: "The type of mission",
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
		Description: "Submit your mission answer",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "answer",
				Description: "Your answer to the mission question",
				Required:    true,
			},
		},
	},
	{
		Name:        "end-mission",
		Description: "End current mission (admin)",
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
		Name:        "force-recruit",
		Description: "Force-recruit a player as a traitor (admin)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "player",
				Description: "The player to forcibly recruit as a traitor",
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
				Name:        "breakfast",
				Description: "Breakfast phase timer in minutes",
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "roundtable",
				Description: "Round Table phase timer in minutes",
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "night",
				Description: "Night phase timer in minutes",
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "mission",
				Description: "Mission phase timer in minutes",
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
	{
		Name:        "rules",
		Description: "Show the full detailed rules of The Traitors",
	},
	{
		Name:        "set-buyin",
		Description: "Set the buy-in amount for this game (admin, lobby only)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "amount",
				Description: "Dollar amount (e.g. 5 or 10.50)",
				Required:    true,
			},
		},
	},
	{
		Name:        "wallet",
		Description: "Share your payment info with losers (winners only, post-game)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "info",
				Description: "Your payment info (e.g. @venmo-handle, PayPal email)",
				Required:    true,
			},
		},
	},
	{
		Name:        "mark-paid",
		Description: "Mark that a loser has paid you (winners only, post-game)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "player",
				Description: "The player who paid you",
				Required:    true,
			},
		},
	},
	{
		Name:        "payment-status",
		Description: "View payment status for this game",
	},
	{
		Name:        "nuke-games",
		Description: "End ALL active/lobby games in this server (admin)",
	},
}
