package main

import (
	"database/sql"
	"time"
)

const dateLayout = "2006-01-02"

// updateStreak applies today's commit status to the user's per-repo streak.
// A single missed day is forgiven as a grace day; the streak only resets to 0
// once 2 consecutive days pass with no commit. It returns the resulting streak.
func updateStreak(db *sql.DB, userID string, repoID int, today string, hadCommitToday bool) (int, error) {
	currentStreak, lastCommitDate, err := getStreak(db, userID, repoID)
	if err != nil {
		return 0, err
	}

	gap := -1
	if lastCommitDate != "" {
		last, err := time.Parse(dateLayout, lastCommitDate)
		if err != nil {
			return 0, err
		}
		todayDate, err := time.Parse(dateLayout, today)
		if err != nil {
			return 0, err
		}
		gap = int(todayDate.Sub(last).Hours() / 24)
	}

	if hadCommitToday {
		var newStreak int
		switch {
		case lastCommitDate == "":
			newStreak = 1
		case gap <= 0:
			newStreak = currentStreak
		case gap <= 2:
			newStreak = currentStreak + 1
		default:
			newStreak = 1
		}
		if err := setStreak(db, userID, repoID, newStreak, today); err != nil {
			return 0, err
		}
		return newStreak, nil
	}

	if lastCommitDate != "" && gap > 2 && currentStreak != 0 {
		if err := setStreak(db, userID, repoID, 0, lastCommitDate); err != nil {
			return 0, err
		}
		return 0, nil
	}

	return currentStreak, nil
}
