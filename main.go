package main

import (
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

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(r.Body)
	log.Printf("Received webhook: %s", string(body))
	w.WriteHeader(http.StatusOK)
}

func main() {
	dg, err := discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}

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

	go func() {
		http.HandleFunc("/webhook", handleWebhook)
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
			if now.After(target) {
				target = target.Add(24 * time.Hour)
			}

			time.Sleep(time.Until(target))

			// check Github commits for today
			// if there are commits, send a message to the Discord channel
			// if there are no commits, do nothing
			_, err := dg.ChannelMessageSend(ChannelID, "Daily commit check: No commits found for today.")
			if err != nil {
				log.Printf("Error sending message: %v", err)
			}
		}
	}()

	log.Println("Bot is now running. Press CTRL-C to exit.")

	select {}
}
