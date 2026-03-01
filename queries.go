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

func getAllRegisteredUserIDs(db *sql.DB) ([]struct{ UserID, ChannelID string }, error) {
	rows, err := db.Query(`
		SELECT rr.user_id, rr.channel_id, r.owner, r.name
		FROM repo_registrations rr
		JOIN repos r ON rr.repo_id = r.id
		JOIN users u ON rr.user_id = u.id
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

	var users []struct{ UserID, ChannelID string }
	for rows.Next() {
		var user struct{ UserID, ChannelID string }
		if err := rows.Scan(&user.UserID, &user.ChannelID); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		users = append(users, user)
	}
	return users, nil
}

func getReposByUserID(db *sql.DB, userID string) ([]struct{ Owner, Name, ChannelID string }, error) {
	rows, err := db.Query(`
		SELECT r.owner, r.name, rr.channel_id
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

	var results []struct{ Owner, Name, ChannelID string }
	for rows.Next() {
		var owner, name, channelID string
		if err := rows.Scan(&owner, &name, &channelID); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		results = append(results, struct{ Owner, Name, ChannelID string }{Owner: owner, Name: name, ChannelID: channelID})
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
