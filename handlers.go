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
	log.Printf("Received webhook: %s", string(body))

	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		log.Printf("Missing signature header")
		http.Error(w, "Missing signature", http.StatusBadRequest)
		return
	}

	if !verifySignature(WebhookSecret, body, signature) {
		log.Printf("Invalid signature: %s", signature)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	var payload PushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Error parsing JSON: %v", err)
	}
	if len(payload.Commits) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	owner := payload.Commits[0].Author.Name
	repo := payload.Repository.Name

	users, err := getUserIDsByRepo(db, owner, repo)
	if err != nil {
		log.Printf("Error getting user ID by owner: %v", err)
	}

	for _, user := range users {
		sendMessage(dg, user.ChannelID, fmt.Sprintf("<@%s> New commit by %s in repo %s: %s", user.UserID, owner, repo, payload.Commits[0].Message))
	}
	w.WriteHeader(http.StatusOK)
}

func handleGithubCallback(db *sql.DB, w http.ResponseWriter, r *http.Request) {
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

	log.Printf("Successfully authenticated user %s for repo %s/%s", pending.DiscordUserID, pending.Owner, pending.Repo)
}
