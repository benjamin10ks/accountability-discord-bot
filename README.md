# Accountability Discord Bot

A Discord bot that keeps you honest about your GitHub activity. Link a repo, and the bot watches your commits, nags you daily, and tracks streaks so you actually keep showing up.

## Features

- **`/register owner/repo`** — links your GitHub account via OAuth and starts tracking a repository for your Discord user.
- **`/unregister owner/repo`** — stops tracking a repository (and removes the GitHub webhook if no one else is tracking it).
- **Real-time commit notifications** — a GitHub webhook posts to the channel whenever you push to a tracked repo.
- **Daily check-in** — every day at 8 PM, the bot checks each tracked repo for commits made that day and posts a per-repo ✅/❌ summary with your current streak.
- **Streaks** — a streak increments for each day you commit and is forgiven for a single missed day, but resets after two consecutive days of inactivity.
- **Weekly report** — every Sunday, the bot posts an activity graphic showing active days and streaks for each tracked repo over the past week.

## Setup

1. Create a Discord application/bot and a GitHub OAuth App.
2. Copy `.env.example` to `.env` and fill in:
   - `DISCORD_BOT_TOKEN`
   - `GITHUB_CLIENT_ID`
   - `GITHUB_CLIENT_SECRET`
   - `BASE_URL` — the publicly reachable URL where this bot is hosted (used for the OAuth callback and GitHub webhook).
   - `WEBHOOK_SECRET` — secret used to verify incoming GitHub webhook payloads.
3. Run the bot:
   ```sh
   go run .
   ```

The bot stores its data in a local SQLite database (`bot.db`) and runs an HTTP server on `:8080` to handle the GitHub OAuth callback and webhook events.
