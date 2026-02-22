package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	BotToken  = os.Getenv("DISCORD_BOT_TOKEN")
	ChannelID = os.Getenv("DISCORD_CHANNEL_ID")
)

type PushPayload struct {
	Commits []struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commits"`
}

type CommitResponse []struct {
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
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

func handleWebhook(dg *discordgo.Session, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
	}
	log.Printf("Received webhook: %s", string(body))

	var payload PushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
	}
	sendMessage(dg, ChannelID, fmt.Sprintf("New commit by %s: %s", payload.Commits[0].Author.Name, payload.Commits[0].Message))
	w.WriteHeader(http.StatusOK)
}

func checkDailyCommits() (*CommitResponse, error) {
	// TODO make username and repo dynamic
	username := "benjamin10ks"
	repo := "accountability-discord-bot"
	since := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	URL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?since=%s", username, repo, since)
	res, err := http.Get(URL)
	if err != nil {
		return nil, fmt.Errorf("error making http request: %v", err)
	}
	defer func() {
		err := res.Body.Close()
		if err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	var commits CommitResponse
	if err = json.Unmarshal(data, &commits); err != nil {
		return nil, fmt.Errorf("error parsing json: %v", err)
	}

	return &commits, nil
}

func main() {
	dg, err := discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

	runMigrations()

	// Registers commands
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		switch i.ApplicationCommandData().Name {
		case "register":
			repoInput := i.ApplicationCommandData().Options[0].StringValue()
			userID := i.Member.User.ID

			log.Printf("Registering repo '%s' for user %s", repoInput, userID)

			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("<@%s> Now watching %s", userID, repoInput),
				},
			})
			if err != nil {
				log.Printf("Error responding to interaction: %v", err)
			}
		}
	})

	err = dg.Open()
	if err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}
	defer func() {
		err := dg.Close()
		if err != nil {
			log.Printf("Error closing Discord session: %v", err)
		}
	}()

	appID := dg.State.User.ID

	for _, cmd := range commands {
		_, err := dg.ApplicationCommandCreate(appID, "", cmd)
		if err != nil {
			log.Fatalf("Cannot create '%v' command: %v", cmd.Name, err)
		}
	}

	go func() {
		http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
			handleWebhook(dg, w, r)
		})
		log.Println("Starting webhook server on :8080")
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Fatalf("Error starting HTTP server: %v", err)
		}
	}()

	go func() {
		for {
			now := time.Now()
			target := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, now.Location())
			// testing 1 minute
			// target := time.Now().Add(1 * time.Minute)
			if now.After(target) {
				target = target.Add(24 * time.Hour)
			}

			time.Sleep(time.Until(target))

			commits, err := checkDailyCommits()
			if err != nil {
				log.Printf("Error checking commits: %v", err)
			}
			if len(*commits) > 0 {
				sendMessage(dg, ChannelID, fmt.Sprintf("Daily commit check: %d commits found for today!", len(*commits)))
			} else {
				sendMessage(dg, ChannelID, "Ur a bum get on it")
			}

		}
	}()

	log.Println("Bot is now running.")

	select {}
}
