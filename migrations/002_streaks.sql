CREATE TABLE streaks (
    user_id TEXT NOT NULL REFERENCES users(id),
    repo_id INTEGER NOT NULL REFERENCES repos(id),
    current_streak INTEGER NOT NULL DEFAULT 0,
    last_commit_date TEXT,
    PRIMARY KEY (user_id, repo_id)
);
