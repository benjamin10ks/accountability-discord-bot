package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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

			parts := strings.Split(repoInput, "/")
			if len(parts) != 2 {
				log.Printf("Invalid repo format: %s", repoInput)
				return
			}

			owner := parts[0]
			repo := parts[1]

			db, err := sql.Open("sqlite3", "./bot.db")
			if err != nil {
				log.Printf("Error opening database: %v", err)
			}

			_, err = db.Exec(`
				INSERT OR REPLACE INTO repo_registrations (discord_user_id, owner, repo_name) 
				VALUES (?, ?, ?)
				ON CONFLICT(discord_user_id) 
				DO UPDATE SET owner=excluded.owner, repo_name=excluded.repo_name`,
				userID, owner, repo)
			if err != nil {
				log.Printf("Error inserting/updating repo registration: %v", err)
			}
			err = db.Close()
			if err != nil {
				log.Printf("Error closing database: %v", err)
			}

			log.Printf("Registering repo '%s' for user %s", repoInput, userID)

			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

			db, err := sql.Open("sqlite3", "./bot.db")
			if err != nil {
				log.Printf("Error opening database: %v", err)
			}

			rows, err := db.Query("SELECT discord_user_id FROM repo_registrations")
			if err != nil {
				log.Printf("Error querying database: %v", err)
				continue
			}
			for rows.Next() {
				var userID string
				err = rows.Scan(&userID)
				if err != nil {
					log.Printf("Error scanning row: %v", err)
				}
				commits, err := checkDailyCommits(userID)
				if err != nil {
					log.Printf("Error checking commits: %v", err)
				}
				if len(*commits) > 0 {
					sendMessage(dg, ChannelID, fmt.Sprintf("<@%s> Daily commit check: %d commits found for today!", userID, len(*commits)))
				} else {
					sendMessage(dg, ChannelID, fmt.Sprintf("Ur a bum get on it <@%s>", userID))
				}
				err = rows.Close()
				if err != nil {
					log.Printf("Error closing rows: %v", err)
				}
			}

			err = db.Close()
			if err != nil {
				log.Printf("Error closing database: %v", err)
			}
		}
	}()

	log.Println("Bot is now running.")

	select {}
}
