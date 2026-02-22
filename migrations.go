package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func runMigrations() {
	db, err := sql.Open("sqlite3", "./bot.db")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err := db.Close()
		if err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	sql := `
		CREATE TABLE IF NOT EXISTS repo_registrations (
		discord_user_id TEXT PRIMARY KEY,
		owner TEXT NOT NULL,
		repo_name TEXT NOT NULL
		);`

	_, err = db.Exec(sql)
	if err != nil {
		log.Fatal(err)
	}
}
