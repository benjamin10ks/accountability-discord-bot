package main

import (
	"database/sql"
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

func registerCommands(dg *discordgo.Session, db *sql.DB) {
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		log.Printf("Received interaction at: %v", time.Now())
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		switch i.ApplicationCommandData().Name {
		case "register":
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

		case "unregister":
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

			webHookID, shouldDelete, err := unregisterRepo(db, userID, owner, repo)
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("Error unregistering repository: %v", err),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				return
			}

			if shouldDelete {
				token, err := getGithubToken(db, userID)
				if err != nil {
					log.Printf("Error getting GitHub token for user %s: %v", userID, err)
				} else {
					if err := deleteGitHubWebhook(token, owner, repo, webHookID); err != nil {
						log.Printf("Error deleting GitHub webhook for %s/%s: %v", owner, repo, err)
					}
				}
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Successfully unregistered repository %s/%s", owner, repo),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})

		}
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
	{
		Name:        "unregister",
		Description: "Unregister a GitHub repository",
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
