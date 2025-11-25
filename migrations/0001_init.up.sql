CREATE TABLE teams (
  team_name TEXT PRIMARY KEY
);

CREATE TABLE users (
  user_id TEXT PRIMARY KEY,
  username TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  team_name TEXT NULL REFERENCES teams(team_name) ON DELETE SET NULL
);

CREATE TABLE prs (
  pull_request_id TEXT PRIMARY KEY,
  pull_request_name TEXT NOT NULL,
  author_id TEXT NOT NULL REFERENCES users(user_id),
  status TEXT NOT NULL CHECK (status IN ('OPEN','MERGED')) DEFAULT 'OPEN',
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  merged_at TIMESTAMP NULL
);

CREATE TABLE pr_reviewers (
  pull_request_id TEXT NOT NULL REFERENCES prs(pull_request_id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(user_id),
  position SMALLINT NOT NULL CHECK (position IN (1,2)),
  PRIMARY KEY (pull_request_id, position),
  UNIQUE (pull_request_id, user_id)
);

CREATE INDEX prs_author_idx ON prs(author_id);
CREATE INDEX pr_reviewers_user_idx ON pr_reviewers(user_id);
