CREATE TABLE daily_activity (
    user_id TEXT NOT NULL REFERENCES users(id),
    repo_id INTEGER NOT NULL REFERENCES repos(id),
    activity_date TEXT NOT NULL,
    had_commit INTEGER NOT NULL,
    PRIMARY KEY (user_id, repo_id, activity_date)
);
