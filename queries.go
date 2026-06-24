package main

import (
	"database/sql"
	"log"
)

func registerRepo(db *sql.DB, userID, owner, repo, channeltID string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
			INSERT OR IGNORE INTO repos (owner, name) 
			VALUES (?, ?)`,
		owner, repo)
	if err != nil {
		tx.Rollback()
		return err
	}

	var repoID int
	err = tx.QueryRow(`SELECT id FROM repos WHERE owner = ? AND name = ?`, owner, repo).Scan(&repoID)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO repo_registrations (user_id, repo_id, channel_id)
		VALUES (?, ?, ?)`,
		userID, repoID, channeltID)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func getAllRegisteredUserIDs(db *sql.DB) ([]struct{ UserID, ChannelID, Owner, Repo string }, error) {
	rows, err := db.Query(`
		SELECT rr.user_id, rr.channel_id, r.owner, r.name
		FROM repo_registrations rr
		JOIN repos r ON rr.repo_id = r.id
		`)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	var users []struct{ UserID, ChannelID, Owner, Repo string }
	for rows.Next() {
		var user struct{ UserID, ChannelID, Owner, Repo string }
		if err := rows.Scan(&user.UserID, &user.ChannelID, &user.Owner, &user.Repo); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		users = append(users, user)
	}
	return users, nil
}

func getReposByUserID(db *sql.DB, userID string) ([]struct {
	RepoID                 int
	Owner, Name, ChannelID string
}, error) {
	rows, err := db.Query(`
		SELECT r.id, r.owner, r.name, rr.channel_id
		FROM repos r
		JOIN repo_registrations rr ON r.id = rr.repo_id
		WHERE rr.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	var results []struct {
		RepoID                 int
		Owner, Name, ChannelID string
	}
	for rows.Next() {
		var repoID int
		var owner, name, channelID string
		if err := rows.Scan(&repoID, &owner, &name, &channelID); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		results = append(results, struct {
			RepoID                 int
			Owner, Name, ChannelID string
		}{RepoID: repoID, Owner: owner, Name: name, ChannelID: channelID})
	}
	return results, nil
}

func getUserIDsByRepo(db *sql.DB, owner, repo string) ([]struct{ UserID, ChannelID string }, error) {
	rows, err := db.Query(`
		SELECT DISTINCT rr.user_id, rr.channel_id
		FROM repos r
		JOIN repo_registrations rr ON r.id = rr.repo_id
		WHERE r.owner = ? AND r.name = ?`, owner, repo)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	var results []struct{ UserID, ChannelID string }
	for rows.Next() {
		var user struct{ UserID, ChannelID string }
		if err := rows.Scan(&user.UserID, &user.ChannelID); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		results = append(results, user)
	}
	return results, nil
}

func storesGithubToken(db *sql.DB, userID, accessToken string) error {
	_, err := db.Exec(`
		INSERT INTO users (id, github_token)
		VALUES (?, ?)
		ON CONFLICT(id) DO UPDATE SET github_token = excluded.github_token`,
		userID, accessToken)
	return err
}

func getGithubToken(db *sql.DB, userID string) (string, error) {
	var token string
	err := db.QueryRow(`SELECT github_token FROM users WHERE id = ?`, userID).Scan(&token)
	return token, err
}

func storeWebhookID(db *sql.DB, owner, repo string, webhookID int64, secret string) error {
	_, err := db.Exec(`
		UPDATE repos SET webhook_id = ?, webhook_secret = ?
		WHERE owner = ? AND name = ?`,
		webhookID, secret, owner, repo)
	return err
}

func getStreak(db *sql.DB, userID string, repoID int) (currentStreak int, lastCommitDate string, err error) {
	err = db.QueryRow(`
		SELECT current_streak, COALESCE(last_commit_date, '')
		FROM streaks WHERE user_id = ? AND repo_id = ?`,
		userID, repoID).Scan(&currentStreak, &lastCommitDate)
	if err == sql.ErrNoRows {
		return 0, "", nil
	}
	return currentStreak, lastCommitDate, err
}

func setStreak(db *sql.DB, userID string, repoID int, currentStreak int, lastCommitDate string) error {
	_, err := db.Exec(`
		INSERT INTO streaks (user_id, repo_id, current_streak, last_commit_date)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, repo_id) DO UPDATE SET
			current_streak = excluded.current_streak,
			last_commit_date = excluded.last_commit_date`,
		userID, repoID, currentStreak, lastCommitDate)
	return err
}

func recordDailyActivity(db *sql.DB, userID string, repoID int, date string, hadCommit bool) error {
	hadCommitInt := 0
	if hadCommit {
		hadCommitInt = 1
	}
	_, err := db.Exec(`
		INSERT INTO daily_activity (user_id, repo_id, activity_date, had_commit)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, repo_id, activity_date) DO UPDATE SET had_commit = excluded.had_commit`,
		userID, repoID, date, hadCommitInt)
	return err
}

func getWeeklyActivity(db *sql.DB, userID, since string) ([]struct {
	Owner, Name           string
	ActiveDays, TotalDays int
	Streak                int
}, error) {
	rows, err := db.Query(`
		SELECT r.owner, r.name, SUM(da.had_commit), COUNT(*), COALESCE(s.current_streak, 0)
		FROM daily_activity da
		JOIN repos r ON da.repo_id = r.id
		LEFT JOIN streaks s ON s.user_id = da.user_id AND s.repo_id = da.repo_id
		WHERE da.user_id = ? AND da.activity_date >= ?
		GROUP BY r.id`, userID, since)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	var results []struct {
		Owner, Name           string
		ActiveDays, TotalDays int
		Streak                int
	}
	for rows.Next() {
		var owner, name string
		var activeDays, totalDays, streak int
		if err := rows.Scan(&owner, &name, &activeDays, &totalDays, &streak); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		results = append(results, struct {
			Owner, Name           string
			ActiveDays, TotalDays int
			Streak                int
		}{Owner: owner, Name: name, ActiveDays: activeDays, TotalDays: totalDays, Streak: streak})
	}
	return results, nil
}

func unregisterRepo(db *sql.DB, userID, owner, repo string) (webhookID int64, shouldDelete bool, err error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, false, err
	}

	var repoID int
	err = tx.QueryRow(`SELECT id FROM repos WHERE owner = ? AND name = ?`, owner, repo).Scan(&repoID)
	if err != nil {
		tx.Rollback()
		return 0, false, err
	}

	_, err = tx.Exec(`DELETE FROM repo_registrations WHERE user_id = ? AND repo_id = ?`, userID, repoID)
	if err != nil {
		tx.Rollback()
		return 0, false, err
	}

	var remaining int
	tx.QueryRow(`SELECT COUNT(*) FROM repo_registrations WHERE repo_id = ?`, repoID).Scan(&remaining)

	if remaining == 0 {
		tx.QueryRow("SELECT COALESCE(webhook_id, 0) FROM repos WHERE id = ?", repoID).Scan(&webhookID)
		tx.Exec(`DELETE FROM repos WHERE id = ?`, repoID)
		shouldDelete = true
	}

	return webhookID, shouldDelete, tx.Commit()
}
