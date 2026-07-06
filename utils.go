package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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

func processUserCommits(db *sql.DB, dg *discordgo.Session, userID, channelID string) {
	commitStatus, err := checkDailyCommits(db, userID)
	if err != nil {
		log.Printf("Error checking daily commits: %v", err)
		return
	}

	today := time.Now().Format(dateLayout)

	var messageBuilder strings.Builder
	messageBuilder.WriteString(fmt.Sprintf("Daily commit check for <@%s>:\n", userID))

	totalCommitsToday := 0

	for _, repo := range commitStatus {
		repoKey := fmt.Sprintf("%s/%s", repo.Owner, repo.Name)

		if err := recordDailyActivity(db, userID, repo.RepoID, today, repo.HasCommit); err != nil {
			log.Printf("Error recording daily activity for %s: %v", repoKey, err)
		}

		streak, err := updateStreak(db, userID, repo.RepoID, today, repo.HasCommit)
		if err != nil {
			log.Printf("Error updating streak for %s: %v", repoKey, err)
		}

		emoji := "❌"
		if repo.HasCommit {
			emoji = "✅"
			totalCommitsToday++
		}

		streakText := ""
		if streak > 0 {
			streakText = fmt.Sprintf(" 🔥 %d day streak", streak)
		}

		messageBuilder.WriteString(fmt.Sprintf("%s %s%s\n", repoKey, emoji, streakText))
	}

	if totalCommitsToday > 0 {
		messageBuilder.WriteString(fmt.Sprintf("Great job <@%s>! You made %d commits today! Keep it up! 🎉", userID, totalCommitsToday))
	} else {
		messageBuilder.WriteString(fmt.Sprintf("Ur a bum <@%s> get on it 😡", userID))
	}

	sendMessage(dg, channelID, messageBuilder.String())
}

func processWeeklyStats(db *sql.DB, dg *discordgo.Session, userID, channelID string) {
	since := time.Now().AddDate(0, 0, -6).Format(dateLayout)

	stats, err := getWeeklyActivity(db, userID, since)
	if err != nil {
		log.Printf("Error getting weekly activity for user %s: %v", userID, err)
		return
	}
	if len(stats) == 0 {
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📊 Weekly Activity Report",
		Description: fmt.Sprintf("<@%s>'s commit activity for the past 7 days", userID),
		Color:       0x5865F2,
	}

	for _, s := range stats {
		bar := strings.Repeat("🟩", s.ActiveDays) + strings.Repeat("⬛", s.TotalDays-s.ActiveDays)
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  fmt.Sprintf("%s/%s", s.Owner, s.Name),
			Value: fmt.Sprintf("%s\n%d/%d active days • 🔥 %d day streak", bar, s.ActiveDays, s.TotalDays, s.Streak),
		})
	}

	if _, err := dg.ChannelMessageSendEmbed(channelID, embed); err != nil {
		log.Printf("Error sending weekly stats embed for user %s: %v", userID, err)
	}
}

func scheduleDailyChecks(db *sql.DB, dg *discordgo.Session) {
	for {
		now := time.Now()
		target := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, now.Location())
		if now.After(target) {
			target = target.Add(24 * time.Hour)
		}

		log.Printf("Next daily check scheduled at: %s", target.Format(time.RFC1123))
		time.Sleep(time.Until(target))

		users, err := getAllRegisteredUserIDs(db)
		if err != nil {
			log.Printf("Error getting registered user IDs: %v", err)
			continue
		}

		for _, user := range users {
			processUserCommits(db, dg, user.UserID, user.ChannelID)
		}

		if target.Weekday() == time.Sunday {
			for _, user := range users {
				processWeeklyStats(db, dg, user.UserID, user.ChannelID)
			}
		}

		time.Sleep(time.Minute)
	}
}

type RepoCommitStatus struct {
	RepoID    int
	Owner     string
	Name      string
	HasCommit bool
}

func checkDailyCommits(db *sql.DB, userID string) ([]RepoCommitStatus, error) {
	repos, err := getReposByUserID(db, userID)
	if err != nil {
		log.Printf("Error getting repo by user ID: %v", err)
		return nil, err
	}

	token, err := getGithubToken(db, userID)
	if err != nil {
		log.Printf("Error getting GitHub token: %v", err)
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	since := startOfDay.Format(time.RFC3339)

	var statuses []RepoCommitStatus

	for _, repo := range repos {
		repoKey := fmt.Sprintf("%s/%s", repo.Owner, repo.Name)
		URL := fmt.Sprintf("https://api.github.com/repos/%s/commits?since=%s&per_page=1", repoKey, since)

		req, err := http.NewRequest("GET", URL, nil)
		if err != nil {
			return nil, err
		}

		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error making request to GitHub API: %v", err)
		}

		data, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response body: %v", err)
		}

		err = res.Body.Close()
		if err != nil {
			log.Printf("Error closing response body: %v", err)
		}

		var commits []any
		if err = json.Unmarshal(data, &commits); err != nil {
			return nil, fmt.Errorf("error parsing json: %v", err)
		}

		statuses = append(statuses, RepoCommitStatus{
			RepoID:    repo.RepoID,
			Owner:     repo.Owner,
			Name:      repo.Name,
			HasCommit: len(commits) > 0,
		})
	}

	return statuses, nil
}
