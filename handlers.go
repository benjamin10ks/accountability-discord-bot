package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
)

type PushPayload struct {
	Commits []struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commits"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

type CommitResponse []struct {
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
}

func handleWebhook(db *sql.DB, dg *discordgo.Session, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
	}
	log.Printf("Received webhook")

	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		log.Printf("Missing signature header")
		http.Error(w, "Missing signature", http.StatusBadRequest)
		return
	}
	log.Printf("Received signature: %s", signature)

	if !verifySignature(WebhookSecret, body, signature) {
		log.Printf("Invalid signature: %s", signature)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}
	log.Printf("Signature verified successfully")

	var payload PushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if len(payload.Commits) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}
	log.Printf("Parsed payload for repo %s/%s with %d commits", payload.Repository.Owner.Login, payload.Repository.Name, len(payload.Commits))

	owner := payload.Repository.Owner.Login
	repo := payload.Repository.Name

	users, err := getUserIDsByRepo(db, owner, repo)
	if err != nil {
		log.Printf("Error getting user ID by Repo: %v", err)
	}
	log.Printf("Found %d users subscribed to repo %s/%s", len(users), owner, repo)

	for _, user := range users {
		sendMessage(dg, user.ChannelID, fmt.Sprintf("<@%s> New commit by %s in repo %s: %s", user.UserID, owner, repo, payload.Commits[0].Message))
		log.Printf("Sent message to user %s for repo %s/%s in channel %s", user.UserID, owner, repo, user.ChannelID)
	}
	w.WriteHeader(http.StatusOK)
}

func handleGithubCallback(db *sql.DB, dg *discordgo.Session, w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	pendingAuthsMu.Lock()
	pending, ok := pendingAuths[state]
	delete(pendingAuths, state)
	pendingAuthsMu.Unlock()

	if !ok || time.Now().After(pending.ExpiresAt) {
		http.Error(w, "Invalid or expired state parameter", http.StatusBadRequest)
		return
	}

	accessToken, err := exchangeCodeForToken(code)
	if err != nil {
		log.Printf("Error exchanging code for token: %v", err)
		http.Error(w, "Error exchanging code for token", http.StatusInternalServerError)
		return
	}

	err = storesGithubToken(db, pending.DiscordUserID, accessToken)
	if err != nil {
		log.Printf("Error storing GitHub token: %v", err)
		http.Error(w, "Error storing GitHub token", http.StatusInternalServerError)
		return
	}
	webhookURL := fmt.Sprintf("%s/webhook", BaseURL)
	err = createWebhook(db, accessToken, pending.Owner, pending.Repo, webhookURL)
	if err != nil {
		log.Printf("Error creating GitHub webhook: %v", err)
		http.Error(w, "Error creating GitHub webhook", http.StatusInternalServerError)
		return
	}

	err = registerRepo(db, pending.DiscordUserID, pending.Owner, pending.Repo, pending.ChannelID)
	if err != nil {
		log.Printf("Error registering repo: %v", err)
	}
	log.Printf("Registered repo %s/%s for user %s in channel %s", pending.Owner, pending.Repo, pending.DiscordUserID, pending.ChannelID)
	sendMessage(dg, pending.ChannelID, fmt.Sprintf("<@%s> Successfully registered repo %s/%s for tracking!", pending.DiscordUserID, pending.Owner, pending.Repo))

	log.Printf("Successfully authenticated user %s for repo %s/%s", pending.DiscordUserID, pending.Owner, pending.Repo)

	fmt.Fprintf(w, `
    <html>
        <body style="font-family: sans-serif; text-align: center; padding: 40px;">
            <h2>✅ Success!</h2>
            <p>%s/%s is now being tracked. You can close this tab and return to Discord.</p>
        </body>
    </html>
`, pending.Owner, pending.Repo)
}
