package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type PendingAuth struct {
	DiscordUserID string
	Owner         string
	Repo          string
	ChannelID     string
	ExpiresAt     time.Time
}

var (
	pendingAuths   = make(map[string]PendingAuth)
	pendingAuthsMu sync.Mutex
)

func registerCommands(dg *discordgo.Session) {
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		log.Printf("Received interaction at: %v", time.Now())
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		go func() {
			switch i.ApplicationCommandData().Name {
			case "register":
				// generate token and build URL first
				repoInput := i.ApplicationCommandData().Options[0].StringValue()
				userID := i.Member.User.ID
				parts := strings.Split(repoInput, "/")
				if len(parts) != 2 {
					s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Invalid format, please use owner/repo",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}

				owner, repo := parts[0], parts[1]
				stateToken := generateStateToken()

				pendingAuthsMu.Lock()
				pendingAuths[stateToken] = PendingAuth{
					DiscordUserID: userID,
					Owner:         owner,
					Repo:          repo,
					ChannelID:     i.ChannelID,
					ExpiresAt:     time.Now().Add(10 * time.Minute),
				}
				pendingAuthsMu.Unlock()

				authURL := fmt.Sprintf(
					"https://github.com/login/oauth/authorize?client_id=%s&scope=admin:repo_hook&state=%s",
					GithubClientID, stateToken,
				)

				// respond directly, no defer needed since we already have everything ready
				err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Click here to authorize GitHub access: %s\n*(Link expires in 10 minutes)*", authURL),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				if err != nil {
					log.Printf("Error responding to interaction: %v", err)
				}
			}
		}()
	})
}

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "register",
		Description: "Register a GitHub repository to watch",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "repo",
				Description: "Repository in format owner/repo",
				Required:    true,
			},
		},
	},
}
