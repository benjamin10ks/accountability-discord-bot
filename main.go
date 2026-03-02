package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/bwmarrin/discordgo"
)

var (
	BotToken       = os.Getenv("DISCORD_BOT_TOKEN")
	GithubClientID = os.Getenv("GITHUB_CLIENT_ID")
	GithubSecret   = os.Getenv("GITHUB_CLIENT_SECRET")
	BaseURL        = os.Getenv("BASE_URL")
	WebhookSecret  = os.Getenv("WEBHOOK_SECRET")
)

func main() {
	if BotToken == "" || GithubClientID == "" || GithubSecret == "" || BaseURL == "" || WebhookSecret == "" {
		log.Fatal("One or more required environment variables are missing: DISCORD_BOT_TOKEN, GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET, BASE_URL, WEBHOOK_SECRET")
	}

	dg, err := discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}
	log.Println("Discord session created successfully.")

	db, err := sql.Open("sqlite", "./bot.db")
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()
	log.Println("Database connection established successfully.")

	err = runMigrations(db)
	if err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}
	log.Println("Database migrations completed successfully.")

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
	log.Println("Discord session opened successfully.")

	registerCommands(dg, db)
	log.Println("Commands registered successfully.")

	appID := dg.State.User.ID

	for _, cmd := range commands {
		_, err := dg.ApplicationCommandCreate(appID, "", cmd)
		if err != nil {
			log.Fatalf("Cannot create '%v' command: %v", cmd.Name, err)
		}
	}

	http.HandleFunc("/github/callback", func(w http.ResponseWriter, r *http.Request) {
		handleGithubCallback(db, dg, w, r)
	})

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		handleWebhook(db, dg, w, r)
	})

	go func() {
		log.Println("Starting http server on :8080")
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Fatalf("Error starting HTTP server: %v", err)
		}
	}()

	go scheduleDailyChecks(db, dg)
	log.Println("Scheduled daily checks successfully.")

	log.Println("Bot is now running.")

	select {}
}
