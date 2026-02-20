package game

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gatorjuice/async_traitors/db"
	"github.com/gatorjuice/async_traitors/notify"
)

// AssignRoles randomly assigns traitor/faithful roles and DMs each player.
func AssignRoles(database *sql.DB, s *discordgo.Session, gameID int64) error {
	players, err := db.GetAllPlayers(database, gameID)
	if err != nil {
		return err
	}

	if len(players) < 4 {
		return errors.New("need at least 4 players to start")
	}

	// Fisher-Yates shuffle with crypto/rand
	for i := len(players) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return fmt.Errorf("random shuffle: %w", err)
		}
		j := n.Int64()
		players[i], players[j] = players[j], players[i]
	}

	traitorCount := len(players) / 4
	if traitorCount < 1 {
		traitorCount = 1
	}

	// Assign roles
	for idx := range players {
		if idx < traitorCount {
			players[idx].Role = string(RoleTraitor)
		} else {
			players[idx].Role = string(RoleFaithful)
		}
		if err := db.UpdatePlayerRole(database, gameID, players[idx].DiscordID, players[idx].Role); err != nil {
			return err
		}
	}

	// Build traitor name list for DMs
	var traitorNames []string
	for _, p := range players {
		if p.Role == string(RoleTraitor) {
			traitorNames = append(traitorNames, p.DiscordName)
		}
	}

	// DM each player
	for _, p := range players {
		var msg string
		if p.Role == string(RoleTraitor) {
			var others []string
			for _, name := range traitorNames {
				if name != p.DiscordName {
					others = append(others, name)
				}
			}
			msg = "You are a **TRAITOR**! Eliminate the faithful to win."
			if len(others) > 0 {
				msg += "\nYour fellow traitors: " + strings.Join(others, ", ")
			}
		} else {
			msg = "You are **FAITHFUL**! Find and banish the traitors to win."
		}
		notify.SendDM(s, p.DiscordID, msg)
	}

	return nil
}
