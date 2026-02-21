package notify

import (
	"time"

	"github.com/bwmarrin/discordgo"
)

// Color constants for embeds.
const (
	ColorInfo    = 0x3498db // blue
	ColorSuccess = 0x2ecc71 // green
	ColorDanger  = 0xe74c3c // red
	ColorWarning = 0xf39c12 // orange
	ColorNight   = 0x9b59b6 // purple
)

// SendDM sends a direct message to a user.
func SendDM(s *discordgo.Session, userID, content string) error {
	if s == nil {
		return nil
	}
	ch, err := s.UserChannelCreate(userID)
	if err != nil {
		return err
	}
	_, err = s.ChannelMessageSend(ch.ID, content)
	return err
}

// SendChannel sends a message to a channel.
func SendChannel(s *discordgo.Session, channelID, content string) (*discordgo.Message, error) {
	if s == nil {
		return nil, nil
	}
	return s.ChannelMessageSend(channelID, content)
}

// SendEmbed sends a rich embed to a channel.
func SendEmbed(s *discordgo.Session, channelID string, embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	if s == nil {
		return nil, nil
	}
	return s.ChannelMessageSendEmbed(channelID, embed)
}

// SendThread sends a message to a thread.
func SendThread(s *discordgo.Session, threadID, content string) (*discordgo.Message, error) {
	if s == nil {
		return nil, nil
	}
	return s.ChannelMessageSend(threadID, content)
}

// CreateThread creates a private thread in a channel.
func CreateThread(s *discordgo.Session, channelID, name string) (*discordgo.Channel, error) {
	if s == nil {
		return nil, nil
	}
	return s.ThreadStart(channelID, name, discordgo.ChannelTypeGuildPrivateThread, 4320)
}

// AddToThread adds a user to a private thread.
func AddToThread(s *discordgo.Session, threadID, userID string) error {
	if s == nil {
		return nil
	}
	return s.ThreadMemberAdd(threadID, userID)
}

// SendEmbedWithComponents sends a rich embed with message components (e.g. buttons) to a channel.
func SendEmbedWithComponents(s *discordgo.Session, channelID string, embed *discordgo.MessageEmbed, components []discordgo.MessageComponent) (*discordgo.Message, error) {
	if s == nil {
		return nil, nil
	}
	return s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})
}

// GameEmbed constructs a standard embed with the given parameters.
func GameEmbed(title, description string, color int, fields []*discordgo.MessageEmbedField) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Async Traitors",
		},
	}
}
