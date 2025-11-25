CREATE TABLE review_assignments (
  id BIGSERIAL PRIMARY KEY,
  pull_request_id TEXT NOT NULL REFERENCES prs(pull_request_id) ON DELETE CASCADE,
  assigned_user_id TEXT NOT NULL REFERENCES users(user_id),
  action TEXT NOT NULL CHECK (action IN ('AUTO_ASSIGN','REASSIGN','SAFE_REASSIGN')),
  created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX review_assignments_user_idx ON review_assignments(assigned_user_id);
CREATE INDEX review_assignments_pr_idx ON review_assignments(pull_request_id);
