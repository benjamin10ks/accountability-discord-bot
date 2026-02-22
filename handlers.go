package main

import (
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
	userID := dg.State.User.ID

	sendMessage(dg, ChannelID, userID, fmt.Sprintf("New commit by %s: %s", payload.Commits[0].Author.Name, payload.Commits[0].Message))
	w.WriteHeader(http.StatusOK)
}
