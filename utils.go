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

func sendMessage(dg *discordgo.Session, channelID, message string) {
	_, err := dg.ChannelMessageSend(channelID, message)
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
	log.Printf("Sent message: %s", message)
}

func checkDailyCommits() (*CommitResponse, error) {
	db, err := sql.Open("sqlite3", "./bot.db")
	if err != nil {
		log.Printf("Error opening database: %v", err)
	}

	row := db.QueryRow("SELECT owner, repo_name FROM repo_registrations LIMIT 1") // Temporary: Only checks the first registered repo

	var owner, repo string
	err = row.Scan(&owner, &repo)
	if err != nil {
		return nil, fmt.Errorf("no repo registered")
	}

	err = db.Close()
	if err != nil {
		log.Printf("Error closing database: %v", err)
	}

	since := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	URL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?since=%s", owner, repo, since)
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
