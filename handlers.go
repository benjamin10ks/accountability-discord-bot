package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/bwmarrin/discordgo"
)

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

	owner := payload.Commits[0].Author.Name

	db, err := sql.Open("sqlite3", "./bot.db")
	if err != nil {
		log.Printf("Error opening database: %v", err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	var userID string
	err = db.QueryRow("SELECT discord_user_id FROM repo_registrations WHERE owner = ?", owner).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No registered user for owner: %s", owner)
		} else {
			log.Printf("Error querying database: %v", err)
		}
		return
	}

	sendMessage(dg, ChannelID, fmt.Sprintf("<@%s> New commit by %s: %s", userID, owner, payload.Commits[0].Message))
	w.WriteHeader(http.StatusOK)
}
